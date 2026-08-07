package taskview

import (
	"strings"
	"testing"

	"github.com/drjzlyan/karya/internal/cellbuf"
	"github.com/drjzlyan/karya/internal/term"
)

func sampleItems() []Item {
	return []Item{
		{ID: "2026-08-06-a", State: "draft", Title: "First task"},
		{ID: "2026-08-06-b", State: "implementing", Title: "Second task"},
		{ID: "2026-08-06-c", State: "done", Title: "Third task"},
	}
}

func TestBoardLoads(t *testing.T) {
	b := New(func() []Item { return sampleItems() })
	if len(b.items) != 3 {
		t.Fatalf("want 3 items, got %d", len(b.items))
	}
	if b.Selected() != "2026-08-06-a" {
		t.Fatalf("first item should be selected, got %q", b.Selected())
	}
}

func TestBoardNavigation(t *testing.T) {
	b := New(func() []Item { return sampleItems() })
	b.HandleKey(term.RuneKey('j'))
	b.HandleKey(term.RuneKey('j'))
	if b.Selected() != "2026-08-06-c" {
		t.Fatalf("selected = %q want last", b.Selected())
	}
	b.HandleKey(term.RuneKey('j')) // clamp
	if b.Selected() != "2026-08-06-c" {
		t.Fatalf("should clamp at last")
	}
	b.HandleKey(term.RuneKey('k'))
	if b.Selected() != "2026-08-06-b" {
		t.Fatalf("selected = %q want b", b.Selected())
	}
}

func TestBoardRefresh(t *testing.T) {
	n := 1
	b := New(func() []Item {
		if n == 1 {
			n++
			return sampleItems()[:1]
		}
		return sampleItems()
	})
	if len(b.items) != 1 {
		t.Fatalf("initial load = %d want 1", len(b.items))
	}
	b.HandleKey(term.RuneKey('r'))
	if len(b.items) != 3 {
		t.Fatalf("after refresh = %d want 3", len(b.items))
	}
}

func TestBoardReviewRequest(t *testing.T) {
	b := New(func() []Item { return sampleItems() })
	b.HandleKey(term.RuneKey('j'))         // select second
	b.HandleKey(term.Named(term.SymEnter)) // request review
	if got := b.ReviewRequest(); got != "2026-08-06-b" {
		t.Fatalf("ReviewRequest = %q want 2026-08-06-b", got)
	}
	if got := b.ReviewRequest(); got != "" {
		t.Fatalf("ReviewRequest should be consumed, got %q", got)
	}
}

func TestBoardAgentRequest(t *testing.T) {
	b := New(func() []Item { return sampleItems() })
	b.HandleKey(term.RuneKey('a')) // request agent for first
	if got := b.AgentRequest(); got != "2026-08-06-a" {
		t.Fatalf("AgentRequest = %q want 2026-08-06-a", got)
	}
	if got := b.AgentRequest(); got != "" {
		t.Fatalf("AgentRequest should be consumed, got %q", got)
	}
}

func TestBoardLifecycleRequest(t *testing.T) {
	cases := []struct {
		key    term.Key
		selIdx int
		wantOp string
		wantID string
	}{
		{term.RuneKey('s'), 0, "start", "2026-08-06-a"},
		{term.RuneKey('p'), 1, "plan", "2026-08-06-b"},
		{term.RuneKey('i'), 1, "implement", "2026-08-06-b"},
		{term.RuneKey('v'), 2, "verify", "2026-08-06-c"},
		{term.RuneKey('m'), 2, "merge", "2026-08-06-c"},
	}
	for _, tc := range cases {
		b := New(func() []Item { return sampleItems() })
		for i := 0; i < tc.selIdx; i++ {
			b.HandleKey(term.RuneKey('j'))
		}
		b.HandleKey(tc.key)
		req, ok := b.LifecycleRequest()
		if !ok || req.Op != tc.wantOp || req.ID != tc.wantID {
			t.Fatalf("%s: got (%+v,%v) want {%s %s}", tc.key, req, ok, tc.wantOp, tc.wantID)
		}
		if _, ok := b.LifecycleRequest(); ok {
			t.Fatalf("%s: request should be consumed once", tc.key)
		}
	}
}

func TestBoardNewTaskInput(t *testing.T) {
	b := New(func() []Item { return sampleItems() })
	b.HandleKey(term.RuneKey('n')) // enter input mode
	if !b.inputting {
		t.Fatal("n should enter input mode")
	}
	for _, r := range "my-feature" {
		b.HandleKey(term.RuneKey(r))
	}
	b.HandleKey(term.Named(term.SymBackspace)) // "my-featur"
	b.HandleKey(term.RuneKey('e'))             // "my-feature"
	b.HandleKey(term.Named(term.SymEnter))
	req, ok := b.LifecycleRequest()
	if !ok || req.Op != "new" || req.ID != "my-feature" {
		t.Fatalf("new task = (%+v,%v) want {new my-feature}", req, ok)
	}
	if b.inputting {
		t.Fatal("Enter should leave input mode")
	}
}

func TestBoardNewTaskCancel(t *testing.T) {
	b := New(func() []Item { return sampleItems() })
	b.HandleKey(term.RuneKey('n'))
	b.HandleKey(term.RuneKey('x'))
	b.HandleKey(term.Named(term.SymEsc))
	if b.inputting {
		t.Fatal("Esc should cancel input mode")
	}
	if _, ok := b.LifecycleRequest(); ok {
		t.Fatal("cancelled input must not emit a request")
	}
}

func TestBoardInputModeSwallowsKeys(t *testing.T) {
	b := New(func() []Item { return sampleItems() })
	b.HandleKey(term.RuneKey('n'))
	b.HandleKey(term.RuneKey('j')) // a letter of the slug, not navigation
	if b.Selected() != "2026-08-06-a" {
		t.Fatalf("input mode must not navigate; selected=%q", b.Selected())
	}
	if b.input != "j" {
		t.Fatalf("input = %q want \"j\"", b.input)
	}
}

func TestBoardQuitCloses(t *testing.T) {
	b := New(func() []Item { return nil })
	if b.Done() {
		t.Fatal("should start open")
	}
	b.HandleKey(term.RuneKey('q'))
	if !b.Done() {
		t.Fatal("q should close")
	}
}

func TestBoardViewRenders(t *testing.T) {
	b := New(func() []Item { return sampleItems() })
	buf := cellbuf.New(60, 10)
	b.View(buf, cellbuf.Rect{X: 0, Y: 0, W: 60, H: 10}, true)
	out := buf.String()
	if !strings.Contains(out, "Tasks (3)") {
		t.Fatalf("header missing:\n%s", out)
	}
	for _, want := range []string{"draft", "First task", "implementing", "done"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q:\n%s", want, out)
		}
	}
}

func TestBoardViewEmpty(t *testing.T) {
	b := New(func() []Item { return nil })
	buf := cellbuf.New(60, 6)
	b.View(buf, cellbuf.Rect{X: 0, Y: 0, W: 60, H: 6}, true)
	if !strings.Contains(buf.String(), "no tasks") {
		t.Fatalf("empty state missing:\n%s", buf.String())
	}
}
