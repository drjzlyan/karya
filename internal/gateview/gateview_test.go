package gateview

import (
	"strings"
	"testing"

	"github.com/drjzlyan/karya/internal/cellbuf"
	"github.com/drjzlyan/karya/internal/term"
)

func items() []Item {
	return []Item{
		{ID: "t-a", State: "planned", Gate: "plan", Title: "First"},
		{ID: "t-b", State: "implementing", Gate: "diff", Title: "Second"},
	}
}

func TestInboxLoadsAndNavigates(t *testing.T) {
	b := New(func() []Item { return items() })
	if len(b.items) != 2 {
		t.Fatalf("want 2 items, got %d", len(b.items))
	}
	b.HandleKey(term.RuneKey('j'))
	if b.sel != 1 {
		t.Fatalf("sel = %d want 1", b.sel)
	}
}

func TestInboxOpenRequest(t *testing.T) {
	b := New(func() []Item { return items() })
	b.HandleKey(term.RuneKey('j'))         // select t-b
	b.HandleKey(term.Named(term.SymEnter)) // request open
	if got := b.OpenRequest(); got != "t-b" {
		t.Fatalf("OpenRequest = %q want t-b", got)
	}
	// consumed once
	if got := b.OpenRequest(); got != "" {
		t.Fatalf("OpenRequest should be consumed, got %q", got)
	}
}

func TestInboxClose(t *testing.T) {
	b := New(func() []Item { return items() })
	b.HandleKey(term.RuneKey('q'))
	if !b.Done() {
		t.Fatal("q should close")
	}
}

func TestInboxView(t *testing.T) {
	b := New(func() []Item { return items() })
	buf := cellbuf.New(60, 8)
	b.View(buf, cellbuf.Rect{X: 0, Y: 0, W: 60, H: 8}, true)
	out := buf.String()
	if !strings.Contains(out, "Gate inbox (2)") || !strings.Contains(out, "gate:plan") || !strings.Contains(out, "First") {
		t.Fatalf("inbox render wrong:\n%s", out)
	}
}

func TestInboxEmpty(t *testing.T) {
	b := New(func() []Item { return nil })
	buf := cellbuf.New(60, 5)
	b.View(buf, cellbuf.Rect{X: 0, Y: 0, W: 60, H: 5}, true)
	if !strings.Contains(buf.String(), "nothing awaiting a gate") {
		t.Fatalf("empty inbox render wrong:\n%s", buf.String())
	}
}
