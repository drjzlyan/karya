package keymap

import (
	"testing"

	"github.com/drjzlyan/karya/internal/term"
)

func testEngine() *Engine { return New(DefaultBindings()) }

func TestForwardWhenNoLeader(t *testing.T) {
	e := testEngine()
	res := e.Feed(term.RuneKey('h'), Context{})
	if res.Kind != ResForward || res.Key != term.RuneKey('h') {
		t.Fatalf("expected forward of 'h', got %+v", res)
	}
}

func TestLeaderThenActionDispatches(t *testing.T) {
	e := testEngine()
	if res := e.Feed(Leader, Context{}); res.Kind != ResPending {
		t.Fatalf("leader should be pending, got %+v", res)
	}
	res := e.Feed(term.RuneKey('l'), Context{})
	if res.Kind != ResDispatch || res.Action != ActionFocusRight {
		t.Fatalf("expected focus-right dispatch, got %+v", res)
	}
	if len(e.Pending()) != 0 {
		t.Fatalf("pending should be cleared after dispatch")
	}
}

func TestTwoLevelGroupChord(t *testing.T) {
	e := testEngine()
	e.Feed(Leader, Context{})
	if res := e.Feed(term.RuneKey('g'), Context{}); res.Kind != ResPending {
		t.Fatalf("leader-g should be pending (group), got %+v", res)
	}
	res := e.Feed(term.RuneKey('g'), Context{})
	if res.Kind != ResDispatch || res.Action != ActionGitPanel {
		t.Fatalf("expected git panel, got %+v", res)
	}
}

func TestSendLiteralLeader(t *testing.T) {
	e := testEngine()
	e.Feed(Leader, Context{})
	res := e.Feed(Leader, Context{})
	if res.Kind != ResDispatch || res.Action != ActionSendLeader {
		t.Fatalf("expected send-leader, got %+v", res)
	}
}

func TestEscCancelsChord(t *testing.T) {
	e := testEngine()
	e.Feed(Leader, Context{})
	e.Feed(term.RuneKey('g'), Context{})
	res := e.Feed(term.Named(term.SymEsc), Context{})
	if res.Kind != ResCancel {
		t.Fatalf("expected cancel, got %+v", res)
	}
	if len(e.Pending()) != 0 {
		t.Fatalf("pending not cleared after cancel")
	}
}

func TestDeadEndChord(t *testing.T) {
	e := testEngine()
	e.Feed(Leader, Context{})
	// leader then an unbound key
	res := e.Feed(term.RuneKey('Z'), Context{})
	if res.Kind != ResNoMatch {
		t.Fatalf("expected no-match, got %+v", res)
	}
	if len(e.Pending()) != 0 {
		t.Fatalf("pending not cleared after dead end")
	}
}

func TestWhichKeyCandidatesTopLevel(t *testing.T) {
	e := testEngine()
	res := e.Feed(Leader, Context{})
	if res.Kind != ResPending {
		t.Fatalf("expected pending, got %+v", res)
	}
	// Expect the groups (t, g) to appear as group candidates and leaves like h.
	var sawGitGroup, sawFocusLeft bool
	for _, c := range res.Pending {
		if c.Key == term.RuneKey('g') && c.IsGroup {
			sawGitGroup = true
		}
		if c.Key == term.RuneKey('h') && !c.IsGroup && c.Desc == "Focus left" {
			sawFocusLeft = true
		}
	}
	if !sawGitGroup {
		t.Fatalf("git group missing from candidates: %+v", res.Pending)
	}
	if !sawFocusLeft {
		t.Fatalf("focus-left leaf missing from candidates")
	}
}

func TestWhichKeyCandidatesUnderGroup(t *testing.T) {
	e := testEngine()
	e.Feed(Leader, Context{})
	res := e.Feed(term.RuneKey('t'), Context{})
	if res.Kind != ResPending {
		t.Fatalf("expected pending under t, got %+v", res)
	}
	want := map[rune]ActionID{'t': ActionTaskBoard, 'n': ActionTaskNew, 's': ActionTaskStart}
	got := map[rune]bool{}
	for _, c := range res.Pending {
		got[c.Rune()] = true
		if c.IsGroup {
			t.Fatalf("task leaves should not be groups: %+v", c)
		}
	}
	for r := range want {
		if !got[r] {
			t.Fatalf("missing task candidate %q in %+v", r, res.Pending)
		}
	}
}

func TestViewSwitchDigits(t *testing.T) {
	e := testEngine()
	e.Feed(Leader, Context{})
	res := e.Feed(term.RuneKey('3'), Context{})
	if res.Kind != ResDispatch || res.Action != ActionViewGit {
		t.Fatalf("expected git view switch, got %+v", res)
	}
}

func TestContextScopedBinding(t *testing.T) {
	// A binding only active when editing.
	bindings := []Binding{
		{Keys: []term.Key{Leader, term.RuneKey('x')}, Action: "editor.only",
			When: func(c Context) bool { return c.Focus == FocusEditor }},
	}
	e := New(bindings)
	// In terminal focus, leader-x is a dead end (binding filtered out).
	e.Feed(Leader, Context{Focus: FocusTerminal})
	if res := e.Feed(term.RuneKey('x'), Context{Focus: FocusTerminal}); res.Kind != ResNoMatch {
		t.Fatalf("binding should not apply in terminal focus, got %+v", res)
	}
	// In editor focus it dispatches.
	e.Reset()
	e.Feed(Leader, Context{Focus: FocusEditor})
	if res := e.Feed(term.RuneKey('x'), Context{Focus: FocusEditor}); res.Kind != ResDispatch {
		t.Fatalf("binding should apply in editor focus, got %+v", res)
	}
}

// Rune is a small test helper on Candidate.
func (c Candidate) Rune() rune { return c.Key.Rune }

func TestParseLeader(t *testing.T) {
	cases := map[string]term.Key{
		"":           term.Ctrl(' '),
		"ctrl+space": term.Ctrl(' '),
		"c-space":    term.Ctrl(' '),
		"ctrl+a":     term.Ctrl('a'),
		"c-b":        term.Ctrl('b'),
		"CTRL+G":     term.Ctrl('g'),
		"ctrl+\\":    term.Ctrl('\\'),
		"nonsense":   term.Ctrl(' '), // falls back to default
	}
	for spec, want := range cases {
		if got := ParseLeader(spec); got != want {
			t.Fatalf("ParseLeader(%q) = %v want %v", spec, got, want)
		}
	}
}

func TestCustomLeaderDispatches(t *testing.T) {
	leader := term.Ctrl('a')
	e := NewWithLeader(DefaultBindingsFor(leader), leader)
	// Ctrl+Space should now just forward (not the leader anymore).
	if res := e.Feed(term.Ctrl(' '), Context{}); res.Kind != ResForward {
		t.Fatalf("old leader should forward now, got %+v", res)
	}
	// Ctrl+A then l -> focus right.
	if res := e.Feed(leader, Context{}); res.Kind != ResPending {
		t.Fatalf("custom leader should be pending, got %+v", res)
	}
	if res := e.Feed(term.RuneKey('l'), Context{}); res.Kind != ResDispatch || res.Action != ActionFocusRight {
		t.Fatalf("custom leader chord failed: %+v", res)
	}
}
