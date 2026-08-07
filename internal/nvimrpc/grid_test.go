package nvimrpc

import (
	"testing"

	"github.com/drjzlyan/karya/internal/cellbuf"
)

// helpers to build synthetic redraw batches (as msgpack-decoded values would be).
func group(name string, insts ...[]any) []any {
	g := []any{name}
	for _, in := range insts {
		g = append(g, in)
	}
	return g
}

func line(row, col int, cells ...[]any) []any {
	cs := make([]any, len(cells))
	for i, c := range cells {
		cs[i] = c
	}
	return []any{int64(1), int64(row), int64(col), cs, false}
}

func cell(text string, args ...int) []any {
	c := []any{text}
	for _, a := range args {
		c = append(c, int64(a))
	}
	return c
}

func TestGridResizeAndLine(t *testing.T) {
	g := NewGrid(4, 2)
	g.HandleRedraw([]any{
		group("grid_resize", []any{int64(1), int64(10), int64(3)}),
		group("grid_line", line(0, 0, cell("h"), cell("i"))),
		group("flush"),
	})
	if w, h := g.Size(); w != 10 || h != 3 {
		t.Fatalf("size = %d,%d", w, h)
	}
	if g.Buffer().Cell(0, 0).Rune != 'h' || g.Buffer().Cell(1, 0).Rune != 'i' {
		t.Fatalf("line not applied: %q", g.Buffer().String())
	}
}

func TestGridFlushReported(t *testing.T) {
	g := NewGrid(4, 1)
	if flushed := g.HandleRedraw([]any{group("grid_line", line(0, 0, cell("x")))}); flushed {
		t.Fatalf("no flush should be reported")
	}
	if flushed := g.HandleRedraw([]any{group("flush")}); !flushed {
		t.Fatalf("flush should be reported")
	}
}

func TestGridCellRepeat(t *testing.T) {
	g := NewGrid(6, 1)
	// "a" then space repeated 3 times.
	g.HandleRedraw([]any{group("grid_line", line(0, 0, cell("a"), cell(" ", 0, 3), cell("b")))})
	if got := g.Buffer().String(); got != "a   b" {
		t.Fatalf("repeat wrong: %q", got)
	}
}

func TestGridCursorGoto(t *testing.T) {
	g := NewGrid(10, 5)
	g.HandleRedraw([]any{group("grid_cursor_goto", []any{int64(1), int64(2), int64(3)})})
	if r, c := g.Cursor(); r != 2 || c != 3 {
		t.Fatalf("cursor = %d,%d want 2,3", r, c)
	}
}

func TestGridModeChange(t *testing.T) {
	g := NewGrid(4, 1)
	g.HandleRedraw([]any{group("mode_change", []any{"insert", int64(1)})})
	if g.Mode() != "insert" {
		t.Fatalf("mode = %q", g.Mode())
	}
}

func TestGridClear(t *testing.T) {
	g := NewGrid(4, 1)
	g.HandleRedraw([]any{group("grid_line", line(0, 0, cell("x"), cell("y")))})
	g.HandleRedraw([]any{group("grid_clear", []any{int64(1)})})
	if got := g.Buffer().String(); got != "" {
		t.Fatalf("clear left content: %q", got)
	}
}

func TestGridHighlightApplied(t *testing.T) {
	g := NewGrid(4, 1)
	g.HandleRedraw([]any{
		group("default_colors_set", []any{int64(0xffffff), int64(0x000000), int64(0), int64(0), int64(0)}),
		group("hl_attr_define", []any{int64(2), map[string]any{"bold": true, "foreground": int64(0xff0000)}, map[string]any{}, []any{}}),
		group("grid_line", line(0, 0, cell("A", 2), cell("b", 0))),
	})
	a := g.Buffer().Cell(0, 0)
	if !a.Style.Attrs.Has(cellbuf.AttrBold) {
		t.Fatalf("A should be bold")
	}
	if a.Style.FG != cellbuf.RGB(0xff, 0, 0) {
		t.Fatalf("A fg = %+v", a.Style.FG)
	}
	// hl 0 -> default fg white, default bg black
	b := g.Buffer().Cell(1, 0)
	if b.Style.FG != cellbuf.RGB(0xff, 0xff, 0xff) || b.Style.BG != cellbuf.RGB(0, 0, 0) {
		t.Fatalf("b default colors wrong: %+v", b.Style)
	}
}

func TestGridScrollUp(t *testing.T) {
	g := NewGrid(3, 3)
	g.HandleRedraw([]any{group("grid_line",
		line(0, 0, cell("a"), cell("a"), cell("a")),
		line(1, 0, cell("b"), cell("b"), cell("b")),
		line(2, 0, cell("c"), cell("c"), cell("c")),
	)})
	// scroll region rows 0..3, cols 0..3, up by 1
	g.HandleRedraw([]any{group("grid_scroll", []any{int64(1), int64(0), int64(3), int64(0), int64(3), int64(1), int64(0)})})
	// row 0 should now be "bbb", row 1 "ccc"
	if g.Buffer().Cell(0, 0).Rune != 'b' || g.Buffer().Cell(0, 1).Rune != 'c' {
		t.Fatalf("scroll up wrong:\n%s", g.Buffer().String())
	}
}

func TestGridWideRune(t *testing.T) {
	g := NewGrid(6, 1)
	g.HandleRedraw([]any{group("grid_line", line(0, 0, cell("世"), cell("x")))}) // col advances by 2 for 世
	if g.Buffer().Cell(0, 0).Rune != '世' || g.Buffer().Cell(0, 0).Width != 2 {
		t.Fatalf("wide rune not applied")
	}
	if g.Buffer().Cell(2, 0).Rune != 'x' {
		t.Fatalf("cell after wide rune wrong: %q", g.Buffer().String())
	}
}
