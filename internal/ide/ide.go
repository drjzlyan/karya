// Package ide wires karya's TUI runtime, unified keymap, and window/pane
// manager into the root model the `karya` program runs (DESIGN.md §6). It owns
// the screen: a tab/pane tree hosting shell (PTY) panes under one Ctrl+Space
// keymap, a status line, and a which-key discovery popup. It is the Phase-1
// walking skeleton — the editor and karya views are added in later phases.
package ide

import (
	"errors"
	"os"
	"os/exec"

	"github.com/drjzlyan/karya/internal/agent"
	"github.com/drjzlyan/karya/internal/cellbuf"
	"github.com/drjzlyan/karya/internal/finder"
	"github.com/drjzlyan/karya/internal/gate"
	"github.com/drjzlyan/karya/internal/gateview"
	"github.com/drjzlyan/karya/internal/git"
	"github.com/drjzlyan/karya/internal/gitui"
	"github.com/drjzlyan/karya/internal/keymap"
	"github.com/drjzlyan/karya/internal/layout"
	"github.com/drjzlyan/karya/internal/review"
	"github.com/drjzlyan/karya/internal/reviewview"
	"github.com/drjzlyan/karya/internal/searchview"
	"github.com/drjzlyan/karya/internal/task"
	"github.com/drjzlyan/karya/internal/taskview"
	"github.com/drjzlyan/karya/internal/term"
	"github.com/drjzlyan/karya/internal/tui"
)

// errNotPending is returned when a gate crossing is attempted on a task that is
// not awaiting a human gate.
var errNotPending = errors.New("task is not awaiting a gate")

// paneView is a karya-native, interactive view pane (git panel, task board, …):
// it renders itself and handles forwarded keys, and can ask to be closed.
type paneView interface {
	layout.PaneContent
	HandleKey(k term.Key)
	Done() bool
}

// spawnFunc creates a pane's content sized for the given inner rectangle. It is
// injected so tests can supply fake panes instead of spawning real shells.
type spawnFunc func(cols, rows int) layout.PaneContent

// Provisioner installs a language's editor tooling (LSP server, formatter, …)
// on demand. karya calls EnsureLanguage in the background when a file of that
// language is opened, so the editor gains language support with no user action.
// It is an interface so the IDE stays decoupled from the tool installer (the CLI
// supplies the implementation).
type Provisioner interface {
	// EnsureLanguage installs the tooling for langName, blocking until done. It
	// must be safe to call from a goroutine and cheap when already installed.
	EnsureLanguage(langName string) error
}

// Model is the root TUI model.
type Model struct {
	tree   *layout.Tree
	keys   *keymap.Engine
	spawn  spawnFunc
	dir    string
	cols   int
	rows   int
	status string

	whichkey     []keymap.Candidate
	shells       []*shellPane
	editors      []*editorPane
	file         string
	leader       term.Key
	prov         Provisioner
	gitPaneID    layout.PaneID
	taskPaneID   layout.PaneID
	reviewPaneID layout.PaneID
	inboxPaneID  layout.PaneID
	finderPaneID layout.PaneID
	searchPaneID layout.PaneID
	editorPaneID layout.PaneID
}

// shellReadMsg carries a chunk of a shell pane's output back into the loop.
type shellReadMsg struct {
	pane *shellPane
	data []byte
	err  error
}

// New builds the root model for working directory dir at the given size, seeding
// the default three-pane view (editor + agent + build/test).
func New(dir string, cols, rows int) *Model {
	m := newModel(dir, cols, rows, nil)
	m.spawn = m.spawnShell
	m.seedDefault()
	return m
}

// seedDefault lays out the ready-to-work default view: editor on the left, a
// coding agent pane top-right, and a build/test shell bottom-right (DESIGN.md
// §6.1). Each pane degrades gracefully when Neovim or an agent CLI is absent.
func (m *Model) seedDefault() {
	inner := m.treeRect()
	right := inner.W / 3 // agent/build share the right third
	left := inner.W - right

	editor := m.spawnEditor("", left-2, inner.H-2)
	agent := m.spawnAgentContent(right-2, inner.H/2-2)
	build := m.spawnShell(right-2, inner.H/2-2)
	m.seedLayout(editor, agent, build)
}

// seedLayout arranges three pane contents into the default view (editor left,
// agent top-right, build bottom-right), adopts each, and focuses the editor. It
// is separated from content creation so it is testable with fake panes.
func (m *Model) seedLayout(editor, agent, build layout.PaneContent) {
	editorID := m.tree.AddTab("dev", editor)
	if _, ok := editor.(*editorPane); ok {
		m.editorPaneID = editorID
	}
	m.adopt(editor)

	m.tree.SplitFocused(layout.SplitH, agent) // editor | agent (focus agent)
	m.adopt(agent)

	m.tree.SplitFocused(layout.SplitV, build) // agent / build (focus build)
	m.adopt(build)

	m.tree.FocusPane(editorID) // start on the editor
	m.syncPaneSizes()
}

// spawnEditor returns an embedded-editor pane for file, or a placeholder when
// Neovim is unavailable.
func (m *Model) spawnEditor(file string, cols, rows int) layout.PaneContent {
	ep, err := newEditorPane(m.dir, file, cols, rows)
	if err != nil {
		return &placeholderPane{title: "editor", body: "Neovim unavailable — Ctrl+Space ? for keys"}
	}
	return ep
}

// spawnAgentContent returns a pane running the first detected agent CLI in the
// repo, or a shell when no agent is installed.
func (m *Model) spawnAgentContent(cols, rows int) layout.PaneContent {
	if name := detectAgentName(""); name != "" {
		if sp, err := newShellPane(exec.Command(name), m.dir, cols, rows); err == nil {
			return sp
		}
	}
	return m.spawnShell(cols, rows)
}

// detectAgentName picks an agent CLI deterministically: the preferred one if
// available, else the first detected (highest preference), else "" (none).
func detectAgentName(preferred string) string {
	if preferred != "" && agent.Available(preferred) {
		return preferred
	}
	if d := agent.Detect(); len(d) > 0 {
		return d[0]
	}
	return ""
}

// NewWithFile builds the root model whose first pane is the embedded editor
// opened on file.
func NewWithFile(dir, file string, cols, rows int) *Model {
	m := newModel(dir, cols, rows, nil)
	m.spawn = m.spawnShell
	m.file = file
	inner := m.treeRect()
	ep, err := newEditorPane(dir, file, inner.W-2, inner.H-2)
	if err != nil {
		return New(dir, cols, rows) // fall back to a shell if nvim is unavailable
	}
	m.editorPaneID = m.tree.AddTab("editor", ep)
	m.adopt(ep)
	m.syncPaneSizes()
	return m
}

// newModel builds a model with an injectable pane factory (used by tests). It
// does not seed an initial pane; call seed after spawn is set.
func newModel(dir string, cols, rows int, spawn spawnFunc) *Model {
	if cols < 1 {
		cols = 80
	}
	if rows < 2 {
		rows = 24
	}
	leader := keymap.ParseLeader(os.Getenv("KARYA_LEADER"))
	return &Model{
		tree:   layout.NewTree(),
		keys:   keymap.NewWithLeader(keymap.DefaultBindingsFor(leader), leader),
		leader: leader,
		spawn:  spawn,
		dir:    dir,
		cols:   cols,
		rows:   rows,
		status: leaderHint(leader),
	}
}

// leaderHint renders the status-line hint naming the active leader.
func leaderHint(leader term.Key) string {
	return leader.String() + " = leader · ? keys · Q quit"
}

// seed creates the first tab's pane and sizes it.
func (m *Model) seed() {
	inner := m.treeRect()
	content := m.spawn(inner.W-2, inner.H-2)
	m.tree.AddTab(paneTitle(content), content)
	m.adopt(content)
	m.syncPaneSizes()
}

// spawnShell is the production spawnFunc: the user's shell, or a placeholder if
// it cannot be started.
func (m *Model) spawnShell(cols, rows int) layout.PaneContent {
	sp, err := defaultShellPane(m.dir, cols, rows)
	if err != nil {
		return &placeholderPane{
			title: "karya",
			body:  "shell unavailable — Ctrl+Space ? for keys, Ctrl+Space Q to quit",
		}
	}
	return sp
}

// adopt registers a pane for sizing/cleanup and returns its background command
// (reading shell output, or waiting for editor redraws).
func (m *Model) adopt(content layout.PaneContent) tui.Cmd {
	switch p := content.(type) {
	case *shellPane:
		m.shells = append(m.shells, p)
		return readCmd(p)
	case *editorPane:
		m.editors = append(m.editors, p)
		return editorWaitCmd(p)
	}
	return nil
}

// treeRect is the region available to the pane tree (all but the status row).
func (m *Model) treeRect() cellbuf.Rect {
	return cellbuf.Rect{X: 0, Y: 0, W: m.cols, H: m.rows - 1}
}

// Init starts reading from every shell pane, waiting on every editor pane, and
// kicks off background language provisioning for any editor whose file's
// language is known.
func (m *Model) Init() tui.Cmd {
	var cmds []tui.Cmd
	for _, sp := range m.shells {
		cmds = append(cmds, readCmd(sp))
	}
	for _, ep := range m.editors {
		cmds = append(cmds, editorWaitCmd(ep))
		if m.prov != nil && ep.lang != "" {
			m.status = "installing " + ep.lang + " language tools…"
			cmds = append(cmds, provisionCmd(m.prov, ep))
		}
	}
	return tui.Batch(cmds...)
}

// provisionCmd installs an editor pane's language tooling in the background.
func provisionCmd(prov Provisioner, ep *editorPane) tui.Cmd {
	return func() tui.Msg {
		err := prov.EnsureLanguage(ep.lang)
		return provisionDoneMsg{pane: ep, lang: ep.lang, err: err}
	}
}

// provisionDoneMsg reports the result of a background language provision.
type provisionDoneMsg struct {
	pane *editorPane
	lang string
	err  error
}

// Update handles a message.
func (m *Model) Update(msg tui.Msg) (tui.Model, tui.Cmd) {
	switch v := msg.(type) {
	case term.KeyEvent:
		return m, m.handleKey(v.Key)
	case term.ResizeEvent:
		m.cols, m.rows = v.Cols, v.Rows
		m.syncPaneSizes()
		return m, nil
	case shellReadMsg:
		return m, m.handleShellRead(v)
	case editorFlushMsg:
		if v.pane.dead {
			return m, nil
		}
		// A repaint happens after every Update; just keep waiting for the next flush.
		return m, editorWaitCmd(v.pane)
	case provisionDoneMsg:
		if v.err != nil {
			m.status = v.lang + " language tools unavailable (see log)"
		} else {
			m.status = v.lang + " language tools ready"
			// The server is now on PATH; re-fire FileType so the LSP attaches.
			if v.pane != nil && !v.pane.dead {
				v.pane.reattachLSP()
			}
		}
		return m, nil
	}
	return m, nil
}

// handleKey routes a key through the unified keymap engine.
func (m *Model) handleKey(k term.Key) tui.Cmd {
	res := m.keys.Feed(k, m.context())
	m.whichkey = nil
	switch res.Kind {
	case keymap.ResForward:
		return m.forward(res.Key)
	case keymap.ResDispatch:
		return m.dispatch(res.Action)
	case keymap.ResPending:
		m.whichkey = res.Pending
		m.status = "leader…  (Esc cancels)"
	case keymap.ResCancel, keymap.ResNoMatch:
		m.status = leaderHint(m.leader)
	}
	return nil
}

// context reports the current focus kind for context-scoped bindings.
func (m *Model) context() keymap.Context {
	switch m.tree.FocusedContent().(type) {
	case *editorPane:
		return keymap.Context{Focus: keymap.FocusEditor}
	case *shellPane:
		return keymap.Context{Focus: keymap.FocusTerminal}
	default:
		return keymap.Context{Focus: keymap.FocusView}
	}
}

// forward sends an unclaimed key to the focused pane: shells receive raw bytes,
// the embedded editor receives Neovim key notation, and karya views handle it
// directly (opening reviews/agents or closing the pane as they ask). It returns
// a command when a view spawns a new backend pane (e.g. an agent shell).
func (m *Model) forward(k term.Key) tui.Cmd {
	switch p := m.tree.FocusedContent().(type) {
	case *editorPane:
		p.write(k)
	case *shellPane:
		p.write(encodeKey(k))
	case paneView:
		id := m.tree.FocusedID()
		p.HandleKey(k)
		// The gate inbox can request opening a task's review.
		if inbox, ok := p.(*gateview.Inbox); ok {
			if openID := inbox.OpenRequest(); openID != "" {
				m.openReviewFor(openID)
				return nil
			}
		}
		// The file finder can request opening a file in the editor.
		if fv, ok := p.(*finder.Finder); ok {
			if path := fv.OpenRequest(); path != "" {
				if m.finderPaneID != 0 && m.tree.FocusPane(m.finderPaneID) {
					m.tree.CloseFocused()
					m.finderPaneID = 0
				}
				cmd := m.openFile(path, 0)
				m.syncPaneSizes()
				return cmd
			}
		}
		// Project search can request opening a match at its location.
		if sv, ok := p.(*searchview.Search); ok {
			if match := sv.OpenRequest(); match != nil {
				if m.searchPaneID != 0 && m.tree.FocusPane(m.searchPaneID) {
					m.tree.CloseFocused()
					m.searchPaneID = 0
				}
				cmd := m.openFile(match.File, match.Line)
				m.syncPaneSizes()
				return cmd
			}
		}
		// The task board can request a review or an agent pane for a task.
		if board, ok := p.(*taskview.Board); ok {
			if rid := board.ReviewRequest(); rid != "" {
				m.openReviewFor(rid)
				return nil
			}
			if aid := board.AgentRequest(); aid != "" {
				return m.openAgentPane(aid)
			}
		}
		if p.Done() {
			switch id {
			case m.gitPaneID:
				m.gitPaneID = 0
			case m.taskPaneID:
				m.taskPaneID = 0
			case m.reviewPaneID:
				m.reviewPaneID = 0
			case m.inboxPaneID:
				m.inboxPaneID = 0
			case m.finderPaneID:
				m.finderPaneID = 0
			case m.searchPaneID:
				m.searchPaneID = 0
			}
			m.tree.CloseFocused()
			m.syncPaneSizes()
		}
	}
	return nil
}

// openAgentPane runs the task's preferred (or first detected) agent CLI in a PTY
// pane bound to the task's worktree, so an interactive agent session works inside
// the task's isolated checkout. Falls back to a shell if no agent CLI is found.
func (m *Model) openAgentPane(id string) tui.Cmd {
	store := task.NewStore(m.dir)
	t, err := store.Get(id)
	if err != nil {
		m.status = "agent: " + err.Error()
		return nil
	}
	if t.Worktree == "" {
		m.status = "start the task first: karya task start " + id
		return nil
	}
	// Pick deterministically (never agent.Resolve — it can prompt on stdin, which
	// would fight the TUI's own input reader).
	name := detectAgentName(t.Agent)
	inner := m.treeRect()
	cols, rows := inner.W-2, inner.H-2

	var sp *shellPane
	title := "shell"
	if name != "" {
		sp, err = newShellPane(exec.Command(name), t.Worktree, cols, rows)
		title = name
	} else {
		sp, err = defaultShellPane(t.Worktree, cols, rows)
	}
	if err != nil {
		m.status = "agent: " + err.Error()
		return nil
	}
	m.tree.AddTab(title+" · "+id, sp)
	cmd := m.adopt(sp)
	m.syncPaneSizes()
	return cmd
}

// openGitPanel focuses the git panel if open, else opens it in a new tab.
func (m *Model) openGitPanel() *gitui.Panel {
	if m.gitPaneID != 0 && m.tree.FocusPane(m.gitPaneID) {
		if p, ok := m.tree.FocusedContent().(*gitui.Panel); ok {
			return p
		}
	}
	panel := gitui.New(git.New(m.dir, nil))
	m.gitPaneID = m.tree.AddTab("git", panel)
	m.syncPaneSizes()
	return panel
}

// openTaskBoard focuses the task board if open, else opens it in a new tab.
func (m *Model) openTaskBoard() *taskview.Board {
	if m.taskPaneID != 0 && m.tree.FocusPane(m.taskPaneID) {
		if b, ok := m.tree.FocusedContent().(*taskview.Board); ok {
			return b
		}
	}
	board := taskview.New(m.loadTasks)
	m.taskPaneID = m.tree.AddTab("tasks", board)
	m.syncPaneSizes()
	return board
}

// openReview opens the review for the first task awaiting a gate.
func (m *Model) openReview() {
	store := task.NewStore(m.dir)
	tasks, err := store.List()
	if err != nil {
		m.status = "review: " + err.Error()
		return
	}
	pending := gate.PendingTasks(tasks)
	if len(pending) == 0 {
		m.status = "no task awaiting review"
		return
	}
	m.openReviewFor(pending[0].ID)
}

// openReviewFor assembles and opens the review for task id, replacing any stale
// review tab. The human can approve/reject from within the review.
func (m *Model) openReviewFor(id string) {
	store := task.NewStore(m.dir)
	rev, err := review.Assemble(store, git.New(m.dir, nil), id)
	if err != nil {
		m.status = "review: " + err.Error()
		return
	}
	if m.reviewPaneID != 0 && m.tree.FocusPane(m.reviewPaneID) {
		m.tree.CloseFocused() // drop the stale review before opening a fresh one
	}
	m.reviewPaneID = m.tree.AddTab("review", reviewview.New(rev, m))
	m.syncPaneSizes()
}

// openFinder opens (or focuses) the fuzzy file finder.
func (m *Model) openFinder() {
	if m.finderPaneID != 0 && m.tree.FocusPane(m.finderPaneID) {
		if _, ok := m.tree.FocusedContent().(*finder.Finder); ok {
			return
		}
	}
	m.finderPaneID = m.tree.AddTab("find", finder.New(finder.ListFiles(m.dir)))
	m.syncPaneSizes()
}

// openSearch opens (or focuses) the project search (live grep) view.
func (m *Model) openSearch() {
	if m.searchPaneID != 0 && m.tree.FocusPane(m.searchPaneID) {
		if _, ok := m.tree.FocusedContent().(*searchview.Search); ok {
			return
		}
	}
	m.searchPaneID = m.tree.AddTab("search", searchview.New(m.dir, searchview.Ripgrep))
	m.syncPaneSizes()
}

// firstEditor returns the first live editor pane, or nil.
func (m *Model) firstEditor() *editorPane {
	for _, ep := range m.editors {
		if !ep.dead {
			return ep
		}
	}
	return nil
}

// focusEditor moves focus to the editor pane, if one exists.
func (m *Model) focusEditor() {
	if m.editorPaneID != 0 {
		m.tree.FocusPane(m.editorPaneID)
	}
}

// openFile opens path (at line; 0 = no jump) in the editor pane, creating one if
// none exists.
func (m *Model) openFile(path string, line int) tui.Cmd {
	if ep := m.firstEditor(); ep != nil {
		m.focusEditor()
		ep.OpenFile(path, line)
		return nil
	}
	// No editor pane yet — open one on the file.
	inner := m.treeRect()
	ed := m.spawnEditor(path, inner.W-2, inner.H-2)
	m.editorPaneID = m.tree.AddTab("editor", ed)
	cmd := m.adopt(ed)
	m.syncPaneSizes()
	return cmd
}

// openInbox focuses the gate inbox if open, else opens it in a new tab.
func (m *Model) openInbox() {
	if m.inboxPaneID != 0 && m.tree.FocusPane(m.inboxPaneID) {
		if _, ok := m.tree.FocusedContent().(*gateview.Inbox); ok {
			return
		}
	}
	m.inboxPaneID = m.tree.AddTab("gates", gateview.New(m.loadGateItems))
	m.syncPaneSizes()
}

// loadGateItems reads the tasks awaiting a gate for the inbox.
func (m *Model) loadGateItems() []gateview.Item {
	store := task.NewStore(m.dir)
	tasks, err := store.List()
	if err != nil {
		return nil
	}
	var items []gateview.Item
	for _, t := range gate.PendingTasks(tasks) {
		p, _ := gate.For(t.State)
		title := t.ID
		if sp, err := store.Spec(t.ID); err == nil {
			title = task.Title(sp)
		}
		items = append(items, gateview.Item{ID: t.ID, State: string(t.State), Gate: string(p.Gate), Title: title})
	}
	return items
}

// Approve crosses the pending gate of task id forward as the human. It satisfies
// reviewview.Crosser so the human can approve from within the review.
func (m *Model) Approve(id string) error { return m.crossGate(id, false, "") }

// Reject sends task id back through its gate with feedback (reviewview.Crosser).
func (m *Model) Reject(id, feedback string) error { return m.crossGate(id, true, feedback) }

// crossGate performs a human gate crossing over the repo's task store.
func (m *Model) crossGate(id string, reject bool, feedback string) error {
	store := task.NewStore(m.dir)
	t, err := store.Get(id)
	if err != nil {
		return err
	}
	p, ok := gate.For(t.State)
	if !ok {
		return errNotPending
	}
	target := p.Approve
	if reject {
		target = p.Reject
	}
	if err := t.Transition(target, "human", feedback); err != nil {
		return err
	}
	return store.Save(t)
}

// loadTasks reads the repo's tasks for the board (decoupling taskview from the
// task store).
func (m *Model) loadTasks() []taskview.Item {
	store := task.NewStore(m.dir)
	tasks, err := store.List()
	if err != nil {
		return nil
	}
	items := make([]taskview.Item, 0, len(tasks))
	for _, t := range tasks {
		title := t.ID
		if sp, err := store.Spec(t.ID); err == nil {
			title = task.Title(sp)
		}
		items = append(items, taskview.Item{ID: t.ID, State: string(t.State), Title: title})
	}
	return items
}

// dispatch runs a karya action.
func (m *Model) dispatch(a keymap.ActionID) tui.Cmd {
	sc := m.treeRect()
	switch a {
	case keymap.ActionFocusLeft:
		m.tree.FocusDir(layout.DirLeft, sc)
	case keymap.ActionFocusDown:
		m.tree.FocusDir(layout.DirDown, sc)
	case keymap.ActionFocusUp:
		m.tree.FocusDir(layout.DirUp, sc)
	case keymap.ActionFocusRight:
		m.tree.FocusDir(layout.DirRight, sc)
	case keymap.ActionResizeLeft:
		m.tree.ResizeFocused(layout.DirLeft)
		m.syncPaneSizes()
	case keymap.ActionResizeRight:
		m.tree.ResizeFocused(layout.DirRight)
		m.syncPaneSizes()
	case keymap.ActionResizeUp:
		m.tree.ResizeFocused(layout.DirUp)
		m.syncPaneSizes()
	case keymap.ActionResizeDown:
		m.tree.ResizeFocused(layout.DirDown)
		m.syncPaneSizes()
	case keymap.ActionSplitRight:
		return m.split(layout.SplitH)
	case keymap.ActionSplitDown:
		return m.split(layout.SplitV)
	case keymap.ActionClosePane:
		return m.closePane()
	case keymap.ActionTabNew:
		return m.newTab()
	case keymap.ActionTabNext:
		m.tree.NextTab()
		m.syncPaneSizes()
	case keymap.ActionTabPrev:
		m.tree.PrevTab()
		m.syncPaneSizes()
	case keymap.ActionTaskBoard:
		m.openTaskBoard()
	case keymap.ActionTaskNew:
		m.status = "new task: run `karya task new <slug>` (in-TUI form coming soon)"
	case keymap.ActionTaskStart:
		m.status = "start task: run `karya task start <id>` (in-TUI form coming soon)"
	case keymap.ActionGitPanel:
		m.openGitPanel()
	case keymap.ActionGitCommit:
		m.openGitPanel().EnterCommit()
	case keymap.ActionGitPush:
		m.openGitPanel().Push()
	case keymap.ActionReview:
		m.openReview()
	case keymap.ActionAgentInbox:
		m.openInbox()
	case keymap.ActionFindFile:
		m.openFinder()
	case keymap.ActionSearch:
		m.openSearch()
	case keymap.ActionFocusEditor:
		m.focusEditor()
	case keymap.ActionSendLeader:
		m.forward(m.leader)
	case keymap.ActionQuit:
		m.shutdown()
		return tui.Quit
	case keymap.ActionHelpKeys:
		l := m.leader.String()
		m.status = l + " then h/j/k/l focus · |/- split · H/J/K/L resize · c/n/p tabs · Q quit"
	default:
		if n := tabGotoIndex(a); n > 0 {
			m.tree.GotoTab(n)
			m.syncPaneSizes()
			break
		}
		m.status = string(a) + " — coming in a later phase"
	}
	return nil
}

// split adds a new pane beside the focused one.
func (m *Model) split(dir layout.SplitDir) tui.Cmd {
	inner := m.treeRect()
	content := m.spawn(inner.W/2, inner.H/2)
	m.tree.SplitFocused(dir, content)
	cmd := m.adopt(content)
	m.syncPaneSizes()
	return cmd
}

// newTab opens a new tab with a fresh pane.
func (m *Model) newTab() tui.Cmd {
	inner := m.treeRect()
	content := m.spawn(inner.W-2, inner.H-2)
	m.tree.AddTab(paneTitle(content), content)
	cmd := m.adopt(content)
	m.syncPaneSizes()
	return cmd
}

// closePane closes the focused pane, tearing down its shell or editor.
func (m *Model) closePane() tui.Cmd {
	switch p := m.tree.FocusedContent().(type) {
	case *shellPane:
		p.close()
		m.removeShell(p)
	case *editorPane:
		p.close()
		m.removeEditor(p)
	}
	m.tree.CloseFocused()
	m.syncPaneSizes()
	return nil
}

// handleShellRead applies shell output to its screen and keeps reading.
func (m *Model) handleShellRead(msg shellReadMsg) tui.Cmd {
	if msg.pane.dead {
		return nil
	}
	if len(msg.data) > 0 {
		_, _ = msg.pane.screen.Write(msg.data)
	}
	if msg.err != nil {
		msg.pane.dead = true
		return nil
	}
	return readCmd(msg.pane)
}

// syncPaneSizes resizes each pane's backend (PTY or Neovim UI) to its current
// inner rectangle.
func (m *Model) syncPaneSizes() {
	for _, p := range m.tree.Compute(m.treeRect()) {
		inner := innerRect(p.Rect)
		switch c := p.Content.(type) {
		case *shellPane:
			c.resize(inner.W, inner.H)
		case *editorPane:
			c.resize(inner.W, inner.H)
		}
	}
}

func (m *Model) removeShell(target *shellPane) {
	for i, sp := range m.shells {
		if sp == target {
			m.shells = append(m.shells[:i], m.shells[i+1:]...)
			return
		}
	}
}

func (m *Model) removeEditor(target *editorPane) {
	for i, ep := range m.editors {
		if ep == target {
			m.editors = append(m.editors[:i], m.editors[i+1:]...)
			return
		}
	}
}

// shutdown tears down all shell and editor processes before the program exits.
func (m *Model) shutdown() {
	for _, sp := range m.shells {
		sp.close()
	}
	for _, ep := range m.editors {
		ep.close()
	}
}

// View renders the whole IDE: framed panes, the status line, and any which-key
// popup.
func (m *Model) View(buf *cellbuf.Buffer) {
	for _, p := range m.tree.Compute(m.treeRect()) {
		inner := drawFrame(buf, p.Rect, paneTitle(p.Content), p.Focused)
		if p.Content != nil {
			p.Content.View(buf, inner, p.Focused)
		}
	}
	m.drawStatus(buf)
	if len(m.whichkey) > 0 {
		m.drawWhichKey(buf)
	}
}

// readCmd returns a command that reads the next chunk from a shell pane.
func readCmd(sp *shellPane) tui.Cmd {
	return func() tui.Msg {
		b := make([]byte, 4096)
		n, err := sp.pty.Read(b)
		data := make([]byte, n)
		copy(data, b[:n])
		return shellReadMsg{pane: sp, data: data, err: err}
	}
}

// editorWaitCmd returns a command that blocks until the editor pane flushes a
// redraw, then asks the program to repaint. It stops when the pane is closed.
func editorWaitCmd(ep *editorPane) tui.Cmd {
	return func() tui.Msg {
		select {
		case <-ep.flush:
			return editorFlushMsg{pane: ep}
		case <-ep.closed:
			return nil
		}
	}
}

// innerRect mirrors drawFrame's inner region for a bordered pane.
func innerRect(r cellbuf.Rect) cellbuf.Rect {
	if r.W < 2 || r.H < 2 {
		return r
	}
	return cellbuf.Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H - 2}
}

func paneTitle(c layout.PaneContent) string {
	switch v := c.(type) {
	case *shellPane:
		return "shell"
	case *editorPane:
		return "editor"
	case *gitui.Panel:
		return "git"
	case *taskview.Board:
		return "tasks"
	case *reviewview.Panel:
		return "review"
	case *gateview.Inbox:
		return "gates"
	case *finder.Finder:
		return "find"
	case *searchview.Search:
		return "search"
	case *placeholderPane:
		return v.title
	}
	return ""
}

func tabGotoIndex(a keymap.ActionID) int {
	switch a {
	case keymap.ActionTabGoto1:
		return 1
	case keymap.ActionTabGoto2:
		return 2
	case keymap.ActionTabGoto3:
		return 3
	case keymap.ActionTabGoto4:
		return 4
	case keymap.ActionTabGoto5:
		return 5
	case keymap.ActionTabGoto6:
		return 6
	case keymap.ActionTabGoto7:
		return 7
	case keymap.ActionTabGoto8:
		return 8
	case keymap.ActionTabGoto9:
		return 9
	}
	return 0
}

// Run launches the IDE against the real terminal for working directory dir. If
// file is non-empty, the first pane is the embedded editor opened on it;
// otherwise it is a shell. prov (may be nil) auto-installs language tooling for
// opened files in the background.
func Run(dir, file string, prov Provisioner) error {
	cols, rows := 80, 24
	fd := int(os.Stdout.Fd())
	if c, r, err := term.Size(fd); err == nil && c > 0 && r > 0 {
		cols, rows = c, r
	}
	caps := term.DetectCaps(os.Getenv)
	var m *Model
	if file != "" {
		m = NewWithFile(dir, file, cols, rows)
	} else {
		m = New(dir, cols, rows)
	}
	m.prov = prov
	prog := tui.NewProgram(m, tui.WithCaps(caps))
	_, err := prog.Run()
	return err
}

var _ tui.Model = (*Model)(nil)
