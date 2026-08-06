package ide

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/drjzlyan/karya/internal/cellbuf"
	"github.com/drjzlyan/karya/internal/keymap"
	"github.com/drjzlyan/karya/internal/layout"
	"github.com/drjzlyan/karya/internal/term"
	"github.com/drjzlyan/karya/internal/tui"
)

// testModel builds a model with fake (non-shell) panes so tests never spawn a
// real process.
func testModel(cols, rows int) *Model {
	m := newModel(".", cols, rows, nil)
	n := 0
	m.spawn = func(c, r int) layout.PaneContent {
		n++
		return &placeholderPane{title: "p"}
	}
	m.seed()
	return m
}

func press(m *Model, keys ...term.Key) tui.Cmd {
	var cmd tui.Cmd
	for _, k := range keys {
		_, cmd = m.Update(term.KeyEvent{Key: k})
	}
	return cmd
}

func leaderThen(runes ...rune) []term.Key {
	keys := []term.Key{keymap.Leader}
	for _, r := range runes {
		keys = append(keys, term.RuneKey(r))
	}
	return keys
}

func focusedRect(m *Model) cellbuf.Rect {
	for _, p := range m.tree.Compute(m.treeRect()) {
		if p.Focused {
			return p.Rect
		}
	}
	return cellbuf.Rect{}
}

func TestSeedHasOnePane(t *testing.T) {
	m := testModel(40, 12)
	if got := len(m.tree.Compute(m.treeRect())); got != 1 {
		t.Fatalf("want 1 pane, got %d", got)
	}
}

func TestLeaderSplitAddsPane(t *testing.T) {
	m := testModel(40, 12)
	press(m, leaderThen('|')...)
	if got := len(m.tree.Compute(m.treeRect())); got != 2 {
		t.Fatalf("after split want 2 panes, got %d", got)
	}
}

func TestLeaderFocusNavigation(t *testing.T) {
	m := testModel(40, 12)
	press(m, leaderThen('|')...) // split right; focus new (right) pane
	rightX := focusedRect(m).X
	press(m, leaderThen('h')...) // focus left
	leftX := focusedRect(m).X
	if leftX >= rightX {
		t.Fatalf("focus did not move left: leftX=%d rightX=%d", leftX, rightX)
	}
}

func TestLeaderResizeChangesGeometry(t *testing.T) {
	m := testModel(40, 12)
	press(m, leaderThen('|')...) // two side-by-side panes, focus right
	before := focusedRect(m).W
	press(m, leaderThen('L')...) // grow right
	after := focusedRect(m).W
	if after <= before {
		t.Fatalf("resize right did not widen focused pane: %d -> %d", before, after)
	}
}

func TestLeaderNewTabAndSwitch(t *testing.T) {
	m := testModel(40, 12)
	press(m, leaderThen('c')...) // new tab
	if m.tree.TabCount() != 2 {
		t.Fatalf("want 2 tabs, got %d", m.tree.TabCount())
	}
	if m.tree.ActiveTab() != 1 {
		t.Fatalf("new tab should be active")
	}
	press(m, leaderThen('1')...) // jump to tab 1
	if m.tree.ActiveTab() != 0 {
		t.Fatalf("goto tab 1 failed, active=%d", m.tree.ActiveTab())
	}
}

func TestLeaderCloseCollapses(t *testing.T) {
	m := testModel(40, 12)
	press(m, leaderThen('|')...) // 2 panes
	press(m, leaderThen('x')...) // close focused
	if got := len(m.tree.Compute(m.treeRect())); got != 1 {
		t.Fatalf("after close want 1 pane, got %d", got)
	}
}

func TestLeaderQuitReturnsCommand(t *testing.T) {
	m := testModel(40, 12)
	cmd := press(m, leaderThen('Q')...)
	if cmd == nil {
		t.Fatalf("quit should return a command")
	}
}

func TestWhichKeyPopulatesOnLeader(t *testing.T) {
	m := testModel(40, 12)
	press(m, keymap.Leader)
	if len(m.whichkey) == 0 {
		t.Fatalf("leader should populate which-key candidates")
	}
	// A subsequent dispatch clears it.
	press(m, term.RuneKey('|'))
	if len(m.whichkey) != 0 {
		t.Fatalf("which-key should clear after dispatch")
	}
}

func TestUnknownActionSetsStatus(t *testing.T) {
	m := testModel(40, 12)
	press(m, leaderThen('a')...) // agent inbox — later phase
	if !strings.Contains(m.status, "later phase") {
		t.Fatalf("expected not-implemented status, got %q", m.status)
	}
}

func TestResizeEventUpdatesSize(t *testing.T) {
	m := testModel(40, 12)
	m.Update(term.ResizeEvent{Cols: 100, Rows: 30})
	if m.cols != 100 || m.rows != 30 {
		t.Fatalf("size not updated: %d x %d", m.cols, m.rows)
	}
}

func TestViewRendersFrameAndStatus(t *testing.T) {
	m := testModel(24, 6)
	buf := cellbuf.New(24, 6)
	m.View(buf)
	out := buf.String()
	if !strings.Contains(out, "┌") || !strings.Contains(out, "┘") {
		t.Fatalf("pane frame not drawn:\n%s", out)
	}
	if !strings.Contains(out, "karya") || !strings.Contains(out, "tab 1/1") {
		t.Fatalf("status line not drawn:\n%s", out)
	}
}

func TestConfigurableLeaderViaEnv(t *testing.T) {
	t.Setenv("KARYA_LEADER", "ctrl+a")
	m := testModel(40, 12)
	// The default Ctrl+Space is no longer the leader; Ctrl+A is.
	press(m, term.Ctrl('a'))
	if len(m.whichkey) == 0 {
		t.Fatalf("Ctrl+A should act as the leader")
	}
	cmd := press(m, term.RuneKey('Q'))
	if cmd == nil {
		t.Fatalf("Ctrl+A then Q should quit")
	}
}

func TestLangForFile(t *testing.T) {
	cases := map[string]string{
		"main.go": "go", "app.py": "python", "lib.rs": "rust",
		"x.ts": "typescript", "x.tsx": "typescript", "x.js": "typescript",
		"m.c": "cpp", "m.cpp": "cpp", "h.hpp": "cpp",
		"README.md": "", "noext": "", "data.json": "",
	}
	for path, want := range cases {
		if got := langForFile(path); got != want {
			t.Fatalf("langForFile(%q) = %q want %q", path, got, want)
		}
	}
}

type fakeProvisioner struct {
	mu     sync.Mutex
	called []string
	err    error
}

func (f *fakeProvisioner) EnsureLanguage(lang string) error {
	f.mu.Lock()
	f.called = append(f.called, lang)
	f.mu.Unlock()
	return f.err
}

func TestProvisionCmdCallsProvisioner(t *testing.T) {
	fp := &fakeProvisioner{}
	ep := &editorPane{lang: "go"}
	msg := provisionCmd(fp, ep)()
	done, ok := msg.(provisionDoneMsg)
	if !ok || done.lang != "go" || done.pane != ep {
		t.Fatalf("unexpected msg: %+v", msg)
	}
	if len(fp.called) != 1 || fp.called[0] != "go" {
		t.Fatalf("provisioner not called for go: %v", fp.called)
	}
}

func TestProvisionDoneUpdatesStatus(t *testing.T) {
	m := testModel(40, 12)
	// success (nil pane is fine — reattach is guarded)
	m.Update(provisionDoneMsg{lang: "go", err: nil})
	if !strings.Contains(m.status, "go language tools ready") {
		t.Fatalf("success status = %q", m.status)
	}
	// failure
	m.Update(provisionDoneMsg{lang: "rust", err: errors.New("boom")})
	if !strings.Contains(m.status, "unavailable") {
		t.Fatalf("failure status = %q", m.status)
	}
}

func TestViewRendersWhichKeyOverlay(t *testing.T) {
	m := testModel(50, 14)
	press(m, keymap.Leader)
	buf := cellbuf.New(50, 14)
	m.View(buf)
	out := buf.String()
	if !strings.Contains(out, "Focus left") {
		t.Fatalf("which-key overlay missing 'Focus left':\n%s", out)
	}
}
