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
