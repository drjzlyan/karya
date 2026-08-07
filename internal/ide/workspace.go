package ide

import (
	"github.com/drjzlyan/karya/internal/cellbuf"
	"github.com/drjzlyan/karya/internal/layout"
	"github.com/drjzlyan/karya/internal/taskview"
	"github.com/drjzlyan/karya/internal/term"
	"github.com/drjzlyan/karya/internal/tui"
)

// WorkspaceKind enumerates karya's six top-level views (DESIGN.md §6). Each is a
// self-contained workspace with its own pane/tab tree; the user switches between
// them with <leader> 1..6 (or the <leader> Space picker).
type WorkspaceKind int

// The six workspaces, in switcher order.
const (
	WSEditor   WorkspaceKind = iota // 1 Human-in-Control: editor + terminal + companion
	WSAgents                        // 2 Multi-Agent software engineering dashboard
	WSGit                           // 3 Git (+ worktrees)
	WSReview                        // 4 Review of pull requests
	WSScratch                       // 5 Scratch pad
	WSSettings                      // 6 Settings
	numWorkspaces
)

// workspaceTitles are the human labels shown in the status line and picker.
var workspaceTitles = [numWorkspaces]string{
	WSEditor:   "editor",
	WSAgents:   "agents",
	WSGit:      "git",
	WSReview:   "review",
	WSScratch:  "scratch",
	WSSettings: "settings",
}

// workspace is one top-level view: its own pane/tab tree plus the pane-IDs of the
// singleton views hosted inside it. Pane IDs are per-workspace because each tree
// allocates ids independently — a global id could collide across workspaces and
// mis-target the forward() close cleanup.
type workspace struct {
	kind   WorkspaceKind
	title  string
	tree   *layout.Tree
	seeded bool // default layout built (lazily, on first activation)

	// singleton view pane IDs, scoped to this workspace's tree
	gitPaneID    layout.PaneID
	taskPaneID   layout.PaneID
	reviewPaneID layout.PaneID
	inboxPaneID  layout.PaneID
	finderPaneID layout.PaneID
	searchPaneID layout.PaneID
	editorPaneID layout.PaneID

	board *taskview.Board // the open task board, for background refresh
}

// initWorkspaces builds the six empty workspaces and activates the editor view.
// The editor workspace is seeded eagerly by New/seed; the rest seed lazily on
// first switch so startup stays fast and stub views cost nothing until visited.
func (m *Model) initWorkspaces() {
	for k := WorkspaceKind(0); k < numWorkspaces; k++ {
		m.workspaces[k] = &workspace{kind: k, title: workspaceTitles[k], tree: layout.NewTree()}
	}
	m.active = WSEditor
	m.tree = m.workspaces[WSEditor].tree
	m.workspaces[WSEditor].seeded = true
}

// ws returns the active workspace.
func (m *Model) ws() *workspace { return m.workspaces[m.active] }

// switchTo makes k the active workspace, lazily seeding its default layout on
// first visit and re-sizing its panes to the current terminal. It returns any
// background command a lazily-seeded pane needs (e.g. reading a new shell).
func (m *Model) switchTo(k WorkspaceKind) tui.Cmd {
	m.dismissPicker()
	if k < 0 || k >= numWorkspaces {
		return nil
	}
	if k == m.active && m.ws().seeded {
		return nil
	}
	m.active = k
	w := m.ws()
	m.tree = w.tree
	var cmd tui.Cmd
	if !w.seeded {
		cmd = m.seedWorkspace(w)
		w.seeded = true
	}
	m.syncPaneSizes()
	m.status = "view " + string(rune('1'+int(k))) + " · " + w.title
	return cmd
}

// seedWorkspace builds a workspace's default layout the first time it is shown.
// Views not yet built (review-of-PRs, scratch depth, settings) get a placeholder
// that later phases replace.
func (m *Model) seedWorkspace(w *workspace) tui.Cmd {
	switch w.kind {
	case WSEditor:
		m.seedDefault() // editor + companion + build
	case WSAgents:
		m.openTaskBoard()
	case WSGit:
		m.openGitPanel()
	case WSScratch:
		inner := m.treeRect()
		content := m.spawnShell(inner.W-2, inner.H-2)
		w.tree.AddTab("scratch", content)
		return m.adopt(content)
	case WSReview:
		w.tree.AddTab("review", &placeholderPane{
			title: "review", body: "Pull-request review — coming in a later phase"})
	case WSSettings:
		w.tree.AddTab("settings", &placeholderPane{
			title: "settings", body: "Settings — coming in a later phase (see " + m.leader.String() + " ? for keys)"})
	}
	return nil
}

// --- view picker overlay ---

// openPicker shows the workspace picker overlay.
func (m *Model) openPicker() {
	m.picker = true
	m.pickerSel = int(m.active)
	m.status = "pick a view: 1-6 / j k / Enter · Esc cancels"
}

// dismissPicker closes the picker overlay if open.
func (m *Model) dismissPicker() {
	if m.picker {
		m.picker = false
		m.status = leaderHint(m.leader)
	}
}

// handlePickerKey processes a key while the picker overlay is open, returning the
// command from any workspace switch it triggers.
func (m *Model) handlePickerKey(k term.Key) tui.Cmd {
	switch {
	case k == term.Named(term.SymEsc) || k == term.RuneKey('q'):
		m.dismissPicker()
	case k == term.RuneKey('j') || k == term.Named(term.SymDown):
		m.pickerSel = (m.pickerSel + 1) % int(numWorkspaces)
	case k == term.RuneKey('k') || k == term.Named(term.SymUp):
		m.pickerSel = (m.pickerSel - 1 + int(numWorkspaces)) % int(numWorkspaces)
	case k == term.Named(term.SymEnter):
		sel := m.pickerSel
		m.dismissPicker()
		return m.switchTo(WorkspaceKind(sel))
	case k.IsRune() && k.Rune >= '1' && k.Rune <= '6':
		sel := int(k.Rune - '1')
		m.dismissPicker()
		return m.switchTo(WorkspaceKind(sel))
	}
	return nil
}

// drawPicker renders the workspace picker overlay centered on screen.
func (m *Model) drawPicker(buf *cellbuf.Buffer) {
	const w = 34
	h := int(numWorkspaces) + 2
	x := max(0, (m.cols-w)/2)
	y := max(0, (m.rows-h)/2)
	box := cellbuf.Rect{X: x, Y: y, W: w, H: h}
	inner := cellbuf.Box(buf, box, "Views", true)
	for k := 0; k < int(numWorkspaces); k++ {
		row := inner.Y + k
		if row >= inner.Y+inner.H {
			break
		}
		st := cellbuf.Style{}
		if k == m.pickerSel {
			st.Attrs |= cellbuf.AttrReverse
		}
		label := " " + string(rune('1'+k)) + "  " + workspaceTitles[k]
		buf.SetString(inner.X, row, fit(label, inner.W), st)
	}
}

// fit truncates s to width w (shared helper for overlays).
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if len(s) > w {
		return s[:w]
	}
	return s
}
