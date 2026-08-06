package layout

import (
	"testing"

	"github.com/drjzlyan/karya/internal/cellbuf"
)

// fakePane is a trivial PaneContent for tests.
type fakePane struct{ label rune }

func (f fakePane) View(buf *cellbuf.Buffer, rect cellbuf.Rect, focused bool) {
	buf.Set(rect.X, rect.Y, cellbuf.Cell{Rune: f.label})
}

func placementByID(ps []Placement, id PaneID) *Placement {
	for i := range ps {
		if ps[i].ID == id {
			return &ps[i]
		}
	}
	return nil
}

func screen(w, h int) cellbuf.Rect { return cellbuf.Rect{X: 0, Y: 0, W: w, H: h} }

func TestSingleTabFullRect(t *testing.T) {
	tr := NewTree()
	id := tr.AddTab("one", fakePane{'A'})
	ps := tr.Compute(screen(20, 10))
	if len(ps) != 1 {
		t.Fatalf("want 1 placement, got %d", len(ps))
	}
	if ps[0].ID != id || ps[0].Rect != screen(20, 10) || !ps[0].Focused {
		t.Fatalf("bad placement %+v", ps[0])
	}
}

func TestSplitHorizontalGeometry(t *testing.T) {
	tr := NewTree()
	a := tr.AddTab("t", fakePane{'A'})
	b := tr.SplitFocused(SplitH, fakePane{'B'})
	ps := tr.Compute(screen(20, 10))
	if len(ps) != 2 {
		t.Fatalf("want 2 panes, got %d", len(ps))
	}
	if r := placementByID(ps, a).Rect; r != (cellbuf.Rect{X: 0, Y: 0, W: 10, H: 10}) {
		t.Fatalf("A rect = %+v", r)
	}
	if r := placementByID(ps, b).Rect; r != (cellbuf.Rect{X: 10, Y: 0, W: 10, H: 10}) {
		t.Fatalf("B rect = %+v", r)
	}
	if !placementByID(ps, b).Focused {
		t.Fatalf("new split pane should be focused")
	}
}

func TestSplitVerticalGeometry(t *testing.T) {
	tr := NewTree()
	a := tr.AddTab("t", fakePane{'A'})
	b := tr.SplitFocused(SplitV, fakePane{'B'})
	ps := tr.Compute(screen(20, 10))
	if r := placementByID(ps, a).Rect; r != (cellbuf.Rect{X: 0, Y: 0, W: 20, H: 5}) {
		t.Fatalf("A rect = %+v", r)
	}
	if r := placementByID(ps, b).Rect; r != (cellbuf.Rect{X: 0, Y: 5, W: 20, H: 5}) {
		t.Fatalf("B rect = %+v", r)
	}
}

// build2x2 makes a 2×2 grid: A left, B top-right, C bottom-right.
func build2x2(t *testing.T) (*Tree, PaneID, PaneID, PaneID) {
	t.Helper()
	tr := NewTree()
	a := tr.AddTab("t", fakePane{'A'})
	b := tr.SplitFocused(SplitH, fakePane{'B'}) // A | B, focus B
	c := tr.SplitFocused(SplitV, fakePane{'C'}) // B over C on the right, focus C
	return tr, a, b, c
}

func TestFocusNavigationGrid(t *testing.T) {
	tr, a, b, c := build2x2(t)
	sc := screen(20, 10)

	// Focus starts at C (bottom-right).
	if tr.FocusedID() != c {
		t.Fatalf("focus = %d want C(%d)", tr.FocusedID(), c)
	}
	// Left from C -> A.
	if !tr.FocusDir(DirLeft, sc) || tr.FocusedID() != a {
		t.Fatalf("left from C should focus A, got %d", tr.FocusedID())
	}
	// Right from A -> B or C (top or bottom right). A spans full height; the
	// nearest by center should be deterministic; just assert it entered the
	// right column.
	if !tr.FocusDir(DirRight, sc) {
		t.Fatalf("right from A should move")
	}
	if id := tr.FocusedID(); id != b && id != c {
		t.Fatalf("right from A should focus right column, got %d", id)
	}
	// From B, down -> C.
	tr.tabs[0].focus = b
	if !tr.FocusDir(DirDown, sc) || tr.FocusedID() != c {
		t.Fatalf("down from B should focus C, got %d", tr.FocusedID())
	}
	// From C, up -> B.
	if !tr.FocusDir(DirUp, sc) || tr.FocusedID() != b {
		t.Fatalf("up from C should focus B, got %d", tr.FocusedID())
	}
}

func TestFocusNavigationNoMoveAtEdge(t *testing.T) {
	tr := NewTree()
	tr.AddTab("t", fakePane{'A'})
	if tr.FocusDir(DirRight, screen(20, 10)) {
		t.Fatalf("single pane should have nowhere to go")
	}
}

func TestResizeGrowsFocused(t *testing.T) {
	tr := NewTree()
	tr.AddTab("t", fakePane{'A'})
	b := tr.SplitFocused(SplitH, fakePane{'B'}) // focus B (right)
	sc := screen(20, 10)
	before := placementByID(tr.Compute(sc), b).Rect.W
	tr.ResizeFocused(DirRight) // grow B's width
	after := placementByID(tr.Compute(sc), b).Rect.W
	if after <= before {
		t.Fatalf("resize right should widen focused pane: before=%d after=%d", before, after)
	}
}

func TestResizeShrinksFocused(t *testing.T) {
	tr := NewTree()
	tr.AddTab("t", fakePane{'A'})
	b := tr.SplitFocused(SplitH, fakePane{'B'})
	sc := screen(20, 10)
	before := placementByID(tr.Compute(sc), b).Rect.W
	tr.ResizeFocused(DirLeft) // shrink focused width
	after := placementByID(tr.Compute(sc), b).Rect.W
	if after >= before {
		t.Fatalf("resize left should narrow focused pane: before=%d after=%d", before, after)
	}
}

func TestCloseFocusedCollapses(t *testing.T) {
	tr := NewTree()
	a := tr.AddTab("t", fakePane{'A'})
	tr.SplitFocused(SplitH, fakePane{'B'}) // focus B
	tr.CloseFocused()                      // remove B
	ps := tr.Compute(screen(20, 10))
	if len(ps) != 1 || ps[0].ID != a {
		t.Fatalf("after close, expected only A full-screen, got %+v", ps)
	}
	if ps[0].Rect != screen(20, 10) {
		t.Fatalf("A should reclaim full screen, got %+v", ps[0].Rect)
	}
}

func TestSiblingSplitDoesNotNest(t *testing.T) {
	tr := NewTree()
	tr.AddTab("t", fakePane{'A'})
	tr.SplitFocused(SplitH, fakePane{'B'}) // A | B
	tr.SplitFocused(SplitH, fakePane{'C'}) // focus B, split H again -> A | B | C siblings
	ps := tr.Compute(screen(30, 10))
	if len(ps) != 3 {
		t.Fatalf("want 3 panes, got %d", len(ps))
	}
	widths := map[int]bool{}
	for _, p := range ps {
		widths[p.Rect.W] = true
		if p.Rect.H != 10 {
			t.Fatalf("sibling split should keep full height, got %+v", p.Rect)
		}
	}
	// three equal columns of 10
	if !widths[10] || len(widths) != 1 {
		t.Fatalf("expected three equal columns, got widths %v", widths)
	}
}

func TestTabsSwitch(t *testing.T) {
	tr := NewTree()
	tr.AddTab("one", fakePane{'A'})
	tr.AddTab("two", fakePane{'B'})
	if tr.TabCount() != 2 || tr.ActiveTab() != 1 {
		t.Fatalf("expected 2 tabs, active 1")
	}
	tr.PrevTab()
	if tr.ActiveTab() != 0 {
		t.Fatalf("prev tab failed")
	}
	tr.GotoTab(2)
	if tr.ActiveTab() != 1 {
		t.Fatalf("goto tab 2 failed")
	}
	tr.NextTab()
	if tr.ActiveTab() != 0 {
		t.Fatalf("next tab should wrap to 0")
	}
}

func TestRenderCallsContent(t *testing.T) {
	tr := NewTree()
	tr.AddTab("t", fakePane{'A'})
	tr.SplitFocused(SplitH, fakePane{'B'})
	buf := cellbuf.New(20, 10)
	tr.Render(buf, screen(20, 10))
	if buf.Cell(0, 0).Rune != 'A' {
		t.Fatalf("A not rendered at its origin")
	}
	if buf.Cell(10, 0).Rune != 'B' {
		t.Fatalf("B not rendered at its origin")
	}
}
