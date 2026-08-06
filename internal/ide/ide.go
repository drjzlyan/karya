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
}

// shellReadMsg carries a chunk of a shell pane's output back into the loop.
type shellReadMsg struct {
	pane *shellPane
	data []byte
	err  error
}

const defaultStatus = "Ctrl+Space = leader · ? keys · Q quit"

// New builds the root model for working directory dir at the given size, using
// real shell panes.
func New(dir string, cols, rows int) *Model {
	m := newModel(dir, cols, rows, nil)
	m.spawn = m.spawnShell
	m.seed()
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
	return &Model{
		tree:   layout.NewTree(),
		keys:   keymap.New(keymap.DefaultBindings()),
		spawn:  spawn,
		dir:    dir,
		cols:   cols,
		rows:   rows,
		status: defaultStatus,
	}
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

// adopt registers a shell pane for sizing/cleanup and returns its read command.
func (m *Model) adopt(content layout.PaneContent) tui.Cmd {
	if sp, ok := content.(*shellPane); ok {
		m.shells = append(m.shells, sp)
		return readCmd(sp)
	}
	return nil
}

// treeRect is the region available to the pane tree (all but the status row).
func (m *Model) treeRect() cellbuf.Rect {
	return cellbuf.Rect{X: 0, Y: 0, W: m.cols, H: m.rows - 1}
}

// Init starts reading from every shell pane.
func (m *Model) Init() tui.Cmd {
	var cmds []tui.Cmd
	for _, sp := range m.shells {
		cmds = append(cmds, readCmd(sp))
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
		m.status = defaultStatus
	}
	return nil
}

// context reports the current focus kind for context-scoped bindings.
func (m *Model) context() keymap.Context {
	switch m.tree.FocusedContent().(type) {
	case *shellPane:
		return keymap.Context{Focus: keymap.FocusTerminal}
	default:
		return keymap.Context{Focus: keymap.FocusView}
	}
}

// forward sends an unclaimed key to the focused pane (only shells accept input
// in the Phase-1 skeleton).
func (m *Model) forward(k term.Key) {
	if sp, ok := m.tree.FocusedContent().(*shellPane); ok {
		sp.write(encodeKey(k))
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
		m.forward(keymap.Leader)
	case keymap.ActionQuit:
		m.shutdown()
		return tui.Quit
	case keymap.ActionHelpKeys:
		m.status = "Ctrl+Space then h/j/k/l focus · |/- split · H/J/K/L resize · c/n/p tabs · Q quit"
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

// closePane closes the focused pane, killing its shell if any.
func (m *Model) closePane() tui.Cmd {
	if sp, ok := m.tree.FocusedContent().(*shellPane); ok {
		sp.close()
		m.removeShell(sp)
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

// syncPaneSizes resizes each shell pane's PTY/emulator to its current inner
// rectangle.
func (m *Model) syncPaneSizes() {
	for _, p := range m.tree.Compute(m.treeRect()) {
		sp, ok := p.Content.(*shellPane)
		if !ok {
			continue
		}
		inner := innerRect(p.Rect)
		sp.resize(inner.W, inner.H)
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

// shutdown kills all shell processes before the program exits.
func (m *Model) shutdown() {
	for _, sp := range m.shells {
		sp.close()
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

// Run launches the IDE against the real terminal for working directory dir.
func Run(dir string) error {
	cols, rows := 80, 24
	fd := int(os.Stdout.Fd())
	if c, r, err := term.Size(fd); err == nil && c > 0 && r > 0 {
		cols, rows = c, r
	}
	caps := term.DetectCaps(os.Getenv)
	m := New(dir, cols, rows)
	prog := tui.NewProgram(m, tui.WithCaps(caps))
	_, err := prog.Run()
	return err
}

var _ tui.Model = (*Model)(nil)
