// Package ide wires karya's TUI runtime, unified keymap, and window/pane
// manager into the root model the `karya` program runs (DESIGN.md §6). It owns
// the screen: a tab/pane tree hosting shell (PTY) panes under one Ctrl+Space
// keymap, a status line, and a which-key discovery popup. It is the Phase-1
// walking skeleton — the editor and karya views are added in later phases.
package ide

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/drjzlyan/karya/internal/agent"
	"github.com/drjzlyan/karya/internal/cellbuf"
	"github.com/drjzlyan/karya/internal/companionview"
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
	"github.com/drjzlyan/karya/internal/tasksvc"
	"github.com/drjzlyan/karya/internal/taskview"
	"github.com/drjzlyan/karya/internal/term"
	"github.com/drjzlyan/karya/internal/tui"
)

// errNoAgent is returned to the companion pane when no coding agent is available.
var errNoAgent = errors.New("no coding agent available")

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
	// The six top-level workspaces and the active one. m.tree always points at
	// the active workspace's pane tree (kept in sync by switchTo/initWorkspaces),
	// so the pane-management code and its tests operate on one live tree.
	workspaces [numWorkspaces]*workspace
	active     WorkspaceKind
	tree       *layout.Tree

	keys   *keymap.Engine
	spawn  spawnFunc
	dir    string
	cols   int
	rows   int
	status string

	whichkey  []keymap.Candidate
	shells    []*shellPane
	editors   []*editorPane
	file      string
	leader    term.Key
	prov      Provisioner
	picker    bool // the workspace picker overlay is open
	pickerSel int  // highlighted view in the picker
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

// seedDefault lays out the ready-to-work default (Human-in-Control) view: editor
// on the left, a read-only Companion agent pane top-right, and a build/test shell
// bottom-right (DESIGN.md §6.1). The companion answers questions but never
// touches files — file-changing agents run headlessly from the Multi-Agent view.
// Each pane degrades gracefully when Neovim or an agent is absent.
func (m *Model) seedDefault() {
	inner := m.treeRect()
	right := inner.W / 3 // companion/build share the right third
	left := inner.W - right

	editor := m.spawnEditor("", left-2, inner.H-2)
	companion := m.spawnCompanion()
	build := m.spawnShell(right-2, inner.H/2-2)
	m.seedLayout(editor, companion, build)
}

// seedLayout arranges three pane contents into the default view (editor left,
// agent top-right, build bottom-right), adopts each, and focuses the editor. It
// is separated from content creation so it is testable with fake panes.
func (m *Model) seedLayout(editor, agent, build layout.PaneContent) {
	editorID := m.tree.AddTab("dev", editor)
	if _, ok := editor.(*editorPane); ok {
		m.ws().editorPaneID = editorID
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
	m.ws().editorPaneID = m.tree.AddTab("editor", ep)
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
	m := &Model{
		keys:   keymap.NewWithLeader(keymap.DefaultBindingsFor(leader), leader),
		leader: leader,
		spawn:  spawn,
		dir:    dir,
		cols:   cols,
		rows:   rows,
		status: leaderHint(leader),
	}
	m.initWorkspaces()
	return m
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
	case lifecycleDoneMsg:
		return m, m.handleLifecycleDone(v)
	case companionAnswerMsg:
		v.comp.Answer(v.text, v.err)
		return m, nil
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

// handleKey routes a key through the unified keymap engine. While the workspace
// picker overlay is open it intercepts keys before the keymap.
func (m *Model) handleKey(k term.Key) tui.Cmd {
	if m.picker {
		return m.handlePickerKey(k)
	}
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
		w := m.ws()
		// The gate inbox can request opening a task's review.
		if inbox, ok := p.(*gateview.Inbox); ok {
			if openID := inbox.OpenRequest(); openID != "" {
				m.openReviewFor(openID)
				return nil
			}
		}
		// The companion pane can ask a headless question (read-only, no edits).
		if comp, ok := p.(*companionview.Companion); ok {
			if q := comp.AskRequest(); q != "" {
				return m.askCompanion(comp, q)
			}
		}
		// The git panel can request opening the selected file in the editor view.
		if panel, ok := p.(*gitui.Panel); ok {
			if path := panel.OpenRequest(); path != "" {
				return m.openFileInEditorView(filepath.Join(m.dir, path), 0)
			}
		}
		// The file finder can request opening a file in the editor.
		if fv, ok := p.(*finder.Finder); ok {
			if path := fv.OpenRequest(); path != "" {
				if w.finderPaneID != 0 && m.tree.FocusPane(w.finderPaneID) {
					m.tree.CloseFocused()
					w.finderPaneID = 0
				}
				cmd := m.openFileInEditorView(path, 0)
				m.syncPaneSizes()
				return cmd
			}
		}
		// Project search can request opening a match at its location.
		if sv, ok := p.(*searchview.Search); ok {
			if match := sv.OpenRequest(); match != nil {
				if w.searchPaneID != 0 && m.tree.FocusPane(w.searchPaneID) {
					m.tree.CloseFocused()
					w.searchPaneID = 0
				}
				cmd := m.openFileInEditorView(match.File, match.Line)
				m.syncPaneSizes()
				return cmd
			}
		}
		// The task board can request a review, a git jump, an agent, or a step.
		if board, ok := p.(*taskview.Board); ok {
			if rid := board.ReviewRequest(); rid != "" {
				m.openReviewFor(rid)
				return nil
			}
			if aid := board.AgentRequest(); aid != "" {
				return m.openAgentPane(aid)
			}
			if gid := board.GitRequest(); gid != "" {
				return m.openGitForTask(gid)
			}
			if req, ok := board.LifecycleRequest(); ok {
				return m.runLifecycle(req)
			}
		}
		if p.Done() {
			switch id {
			case w.gitPaneID:
				w.gitPaneID = 0
			case w.taskPaneID:
				w.taskPaneID = 0
				w.board = nil
			case w.reviewPaneID:
				w.reviewPaneID = 0
			case w.inboxPaneID:
				w.inboxPaneID = 0
			case w.finderPaneID:
				w.finderPaneID = 0
			case w.searchPaneID:
				w.searchPaneID = 0
			}
			m.tree.CloseFocused()
			m.syncPaneSizes()
		}
	}
	return nil
}

// spawnCompanion returns the read-only Companion agent pane for the editor view,
// backed by the first detected agent (or none).
func (m *Model) spawnCompanion() layout.PaneContent {
	return companionview.New(detectAgentName(""))
}

// companionAnswerMsg carries a headless agent's reply back to a companion pane.
type companionAnswerMsg struct {
	comp *companionview.Companion
	text string
	err  error
}

// askCompanion runs a companion question through a headless agent off the render
// path and returns the reply to the pane. The companion never edits files — it
// uses the agent's one-shot headless mode purely to answer (DESIGN.md §6).
func (m *Model) askCompanion(c *companionview.Companion, q string) tui.Cmd {
	name := detectAgentName("")
	dir := m.dir
	return func() tui.Msg {
		if name == "" {
			return companionAnswerMsg{comp: c, err: errNoAgent}
		}
		text, err := agent.NewRunner(name).Headless(context.Background(), dir, q)
		return companionAnswerMsg{comp: c, text: text, err: err}
	}
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

// openGitForTask jumps to the Git view for a task (to inspect its worktree and
// branch). The dedicated worktree list/diff arrives in a later phase.
func (m *Model) openGitForTask(id string) tui.Cmd {
	m.switchTo(WSGit)
	m.openGitPanel()
	m.status = "git · task " + id
	return nil
}

// openGitPanel focuses the git panel if open, else opens it in the Git view.
// The caller switches to WSGit first (dispatch/cross-nav), so it targets that
// workspace's tree.
func (m *Model) openGitPanel() *gitui.Panel {
	w := m.ws()
	if w.gitPaneID != 0 && m.tree.FocusPane(w.gitPaneID) {
		if p, ok := m.tree.FocusedContent().(*gitui.Panel); ok {
			return p
		}
	}
	panel := gitui.New(git.New(m.dir, nil))
	w.gitPaneID = m.tree.AddTab("git", panel)
	m.syncPaneSizes()
	return panel
}

// openTaskBoard focuses the task board if open, else opens it in the Multi-Agent
// view. The caller switches to WSAgents first.
func (m *Model) openTaskBoard() *taskview.Board {
	w := m.ws()
	if w.taskPaneID != 0 && m.tree.FocusPane(w.taskPaneID) {
		if b, ok := m.tree.FocusedContent().(*taskview.Board); ok {
			return b
		}
	}
	board := taskview.New(m.loadTasks)
	w.board = board
	w.taskPaneID = m.tree.AddTab("tasks", board)
	m.syncPaneSizes()
	return board
}

// lifecycleDoneMsg reports the result of a background lifecycle step.
type lifecycleDoneMsg struct {
	req taskview.LifecycleRequest
	id  string // the resolved task id (set for Op=="new")
	err error
}

// runLifecycle drives one gate-lifecycle step from the task board in-process, off
// the render path so the TUI stays responsive (DESIGN.md §6). The step logic
// lives in internal/tasksvc — the same functions the CLI once shelled out to.
// The board's status line reflects progress and, on completion, the result.
func (m *Model) runLifecycle(req taskview.LifecycleRequest) tui.Cmd {
	env, err := tasksvc.RepoEnv(m.dir)
	if err != nil {
		m.status = "lifecycle: " + err.Error()
		return nil
	}
	m.status = req.Op + " " + req.ID + " …"
	return func() tui.Msg {
		id := req.ID
		var runErr error
		switch req.Op {
		case "new":
			t, e := tasksvc.NewTask(env, req.ID, "")
			id, runErr = t.ID, e
		case "start":
			_, runErr = tasksvc.Start(env, req.ID, "HEAD")
		case "plan":
			_, runErr = tasksvc.Plan(context.Background(), env, req.ID)
		case "implement":
			_, runErr = tasksvc.Implement(context.Background(), env, req.ID)
		case "verify":
			res, _, e := tasksvc.Verify(env, req.ID)
			if e == nil && !res.Passed() {
				e = errors.New("verification failed")
			}
			runErr = e
		case "merge":
			runErr = tasksvc.Merge(env, req.ID, false)
		}
		return lifecycleDoneMsg{req: req, id: id, err: runErr}
	}
}

// handleLifecycleDone records a finished lifecycle step: it refreshes the
// Multi-Agent board (states changed on disk), reports success or the failure's
// first line, and for a newly created task opens its spec in the editor to fill
// in the contract.
func (m *Model) handleLifecycleDone(v lifecycleDoneMsg) tui.Cmd {
	board := m.workspaces[WSAgents].board
	if board != nil {
		board.Refresh()
	}
	if v.err != nil {
		msg := fmt.Sprintf("%s %s failed: %s", v.req.Op, v.req.ID, firstLine(v.err.Error()))
		m.status = msg
		if board != nil {
			board.SetStatus(msg)
		}
		return nil
	}
	msg := fmt.Sprintf("%s %s ✓", v.req.Op, v.req.ID)
	m.status = msg
	if board != nil {
		board.SetStatus(msg)
	}
	if v.req.Op == "new" && v.id != "" {
		cmd := m.openFileInEditorView(task.NewStore(m.dir).SpecPath(v.id), 0)
		m.syncPaneSizes()
		return cmd
	}
	return nil
}

// firstLine returns the first non-empty line of s, trimmed (for status errors).
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return ""
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

// openReviewFor assembles and opens the review for task id in the Review view,
// replacing any stale review tab. The human can approve/reject from within it.
func (m *Model) openReviewFor(id string) {
	store := task.NewStore(m.dir)
	rev, err := review.Assemble(store, git.New(m.dir, nil), id)
	if err != nil {
		m.status = "review: " + err.Error()
		return
	}
	m.switchTo(WSReview)
	w := m.ws()
	if w.reviewPaneID != 0 && m.tree.FocusPane(w.reviewPaneID) {
		m.tree.CloseFocused() // drop the stale review before opening a fresh one
	}
	w.reviewPaneID = m.tree.AddTab("review", reviewview.New(rev, m))
	m.syncPaneSizes()
}

// openFinder opens (or focuses) the fuzzy file finder in the editor view.
func (m *Model) openFinder() {
	m.switchTo(WSEditor)
	w := m.ws()
	if w.finderPaneID != 0 && m.tree.FocusPane(w.finderPaneID) {
		if _, ok := m.tree.FocusedContent().(*finder.Finder); ok {
			return
		}
	}
	w.finderPaneID = m.tree.AddTab("find", finder.New(finder.ListFiles(m.dir)))
	m.syncPaneSizes()
}

// openSearch opens (or focuses) the project search (live grep) view in the editor
// view.
func (m *Model) openSearch() {
	m.switchTo(WSEditor)
	w := m.ws()
	if w.searchPaneID != 0 && m.tree.FocusPane(w.searchPaneID) {
		if _, ok := m.tree.FocusedContent().(*searchview.Search); ok {
			return
		}
	}
	w.searchPaneID = m.tree.AddTab("search", searchview.New(m.dir, searchview.Ripgrep))
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

// focusEditor switches to the editor view and focuses its editor pane.
func (m *Model) focusEditor() {
	m.switchTo(WSEditor)
	if id := m.ws().editorPaneID; id != 0 {
		m.tree.FocusPane(id)
	}
}

// openFileInEditorView switches to the editor view and opens path there. It is
// the cross-view entry point used from Git/search/finder/task flows.
func (m *Model) openFileInEditorView(path string, line int) tui.Cmd {
	m.switchTo(WSEditor)
	return m.openFile(path, line)
}

// openFile opens path (at line; 0 = no jump) in the active view's editor pane,
// creating one if none exists. Callers switch to the editor view first.
func (m *Model) openFile(path string, line int) tui.Cmd {
	if ep := m.firstEditor(); ep != nil {
		m.focusEditor()
		ep.OpenFile(path, line)
		return nil
	}
	// No editor pane yet — open one on the file.
	inner := m.treeRect()
	ed := m.spawnEditor(path, inner.W-2, inner.H-2)
	m.ws().editorPaneID = m.tree.AddTab("editor", ed)
	cmd := m.adopt(ed)
	m.syncPaneSizes()
	return cmd
}

// openInbox opens (or focuses) the gate inbox in the Multi-Agent view.
func (m *Model) openInbox() {
	m.switchTo(WSAgents)
	w := m.ws()
	if w.inboxPaneID != 0 && m.tree.FocusPane(w.inboxPaneID) {
		if _, ok := m.tree.FocusedContent().(*gateview.Inbox); ok {
			return
		}
	}
	w.inboxPaneID = m.tree.AddTab("gates", gateview.New(m.loadGateItems))
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
	env, err := tasksvc.RepoEnv(m.dir)
	if err != nil {
		return err
	}
	return tasksvc.CrossGate(env, id, "human", reject, feedback)
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
	case keymap.ActionViewEditor:
		return m.switchTo(WSEditor)
	case keymap.ActionViewAgents:
		return m.switchTo(WSAgents)
	case keymap.ActionViewGit:
		return m.switchTo(WSGit)
	case keymap.ActionViewReview:
		return m.switchTo(WSReview)
	case keymap.ActionViewScratch:
		return m.switchTo(WSScratch)
	case keymap.ActionViewSettings:
		return m.switchTo(WSSettings)
	case keymap.ActionViewPicker:
		m.openPicker()
	case keymap.ActionTaskBoard:
		m.switchTo(WSAgents)
		m.openTaskBoard()
	case keymap.ActionTaskNew:
		m.switchTo(WSAgents)
		m.openTaskBoard().BeginNew()
	case keymap.ActionTaskStart:
		m.switchTo(WSAgents)
		m.openTaskBoard()
		m.status = "select a task and press s to start it"
	case keymap.ActionGitPanel:
		m.switchTo(WSGit)
		m.openGitPanel()
	case keymap.ActionGitCommit:
		m.switchTo(WSGit)
		m.openGitPanel().EnterCommit()
	case keymap.ActionGitPush:
		m.switchTo(WSGit)
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
		m.status = l + " 1-6 views · h/j/k/l focus · |/- split · c/n/p tabs · Q quit"
	default:
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
	if m.picker {
		m.drawPicker(buf)
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
	case *companionview.Companion:
		return "companion"
	case *placeholderPane:
		return v.title
	}
	return ""
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
