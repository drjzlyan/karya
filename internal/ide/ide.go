// Package ide wires karya's TUI runtime, unified keymap, and window/pane
// manager into the root model the `karya` program runs (DESIGN.md §6). It owns
// the screen: a tab/pane tree hosting shell (PTY) panes under one Ctrl+Space
// keymap, a status line, and a which-key discovery popup. It is the Phase-1
// walking skeleton — the editor and karya views are added in later phases.
package ide

import (
	"os"

	"github.com/drjzlyan/karya/internal/cellbuf"
	"github.com/drjzlyan/karya/internal/keymap"
	"github.com/drjzlyan/karya/internal/layout"
	"github.com/drjzlyan/karya/internal/term"
	"github.com/drjzlyan/karya/internal/tui"
)

// spawnFunc creates a pane's content sized for the given inner rectangle. It is
// injected so tests can supply fake panes instead of spawning real shells.
type spawnFunc func(cols, rows int) layout.PaneContent

// Model is the root TUI model.
type Model struct {
	tree   *layout.Tree
	keys   *keymap.Engine
	spawn  spawnFunc
	dir    string
	cols   int
	rows   int
	status string

	whichkey []keymap.Candidate
	shells   []*shellPane
	editors  []*editorPane
	file     string
	leader   term.Key
}

// shellReadMsg carries a chunk of a shell pane's output back into the loop.
type shellReadMsg struct {
	pane *shellPane
	data []byte
	err  error
}

// New builds the root model for working directory dir at the given size. Its
// first pane is a shell.
func New(dir string, cols, rows int) *Model {
	m := newModel(dir, cols, rows, nil)
	m.spawn = m.spawnShell
	m.seed()
	return m
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
	m.tree.AddTab("editor", ep)
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

// Init starts reading from every shell pane and waiting on every editor pane.
func (m *Model) Init() tui.Cmd {
	var cmds []tui.Cmd
	for _, sp := range m.shells {
		cmds = append(cmds, readCmd(sp))
	}
	for _, ep := range m.editors {
		cmds = append(cmds, editorWaitCmd(ep))
	}
	return tui.Batch(cmds...)
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
	}
	return m, nil
}

// handleKey routes a key through the unified keymap engine.
func (m *Model) handleKey(k term.Key) tui.Cmd {
	res := m.keys.Feed(k, m.context())
	m.whichkey = nil
	switch res.Kind {
	case keymap.ResForward:
		m.forward(res.Key)
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
// the embedded editor receives Neovim key notation.
func (m *Model) forward(k term.Key) {
	switch p := m.tree.FocusedContent().(type) {
	case *editorPane:
		p.write(k)
	case *shellPane:
		p.write(encodeKey(k))
	}
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
// otherwise it is a shell.
func Run(dir, file string) error {
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
	prog := tui.NewProgram(m, tui.WithCaps(caps))
	_, err := prog.Run()
	return err
}

var _ tui.Model = (*Model)(nil)
