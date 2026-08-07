package cellbuf

import (
	"reflect"
	"testing"
)

func TestNewSizeAndBlankFill(t *testing.T) {
	b := New(4, 2)
	if w, h := b.Size(); w != 4 || h != 2 {
		t.Fatalf("Size() = %d,%d want 4,2", w, h)
	}
	for y := 0; y < 2; y++ {
		for x := 0; x < 4; x++ {
			if got := b.Cell(x, y); got != (Cell{Rune: ' ', Width: 1}) {
				t.Fatalf("cell(%d,%d) = %+v want blank", x, y, got)
			}
		}
	}
}

func TestNewClampsNegative(t *testing.T) {
	b := New(-3, -1)
	if w, h := b.Size(); w != 0 || h != 0 {
		t.Fatalf("Size() = %d,%d want 0,0", w, h)
	}
}

func TestSetAndCellOutOfBounds(t *testing.T) {
	b := New(2, 2)
	b.Set(0, 0, Cell{Rune: 'x'})
	if b.Cell(0, 0).Rune != 'x' {
		t.Fatalf("set/get roundtrip failed")
	}
	// Set auto-fills Width from the rune.
	if b.Cell(0, 0).Width != 1 {
		t.Fatalf("width = %d want 1", b.Cell(0, 0).Width)
	}
	// Out of bounds Set is ignored; Cell returns blank.
	b.Set(9, 9, Cell{Rune: 'z'})
	if b.Cell(9, 9) != (Cell{Rune: ' ', Width: 1}) {
		t.Fatalf("oob cell not blank")
	}
}

func TestSetStringASCII(t *testing.T) {
	b := New(10, 1)
	next := b.SetString(2, 0, "hi", Style{})
	if next != 4 {
		t.Fatalf("next x = %d want 4", next)
	}
	if b.Cell(2, 0).Rune != 'h' || b.Cell(3, 0).Rune != 'i' {
		t.Fatalf("string not written")
	}
	if b.Cell(0, 0).Rune != ' ' {
		t.Fatalf("leading cell disturbed")
	}
}

func TestSetStringWideRuneOccupiesTwoCells(t *testing.T) {
	b := New(6, 1)
	// '世' is East Asian wide (width 2).
	next := b.SetString(0, 0, "世x", Style{})
	if next != 3 {
		t.Fatalf("next x = %d want 3 (wide=2 + narrow=1)", next)
	}
	lead := b.Cell(0, 0)
	if lead.Rune != '世' || lead.Width != 2 {
		t.Fatalf("lead cell = %+v want 世 width 2", lead)
	}
	cont := b.Cell(1, 0)
	if cont.Rune != 0 || cont.Width != 0 {
		t.Fatalf("continuation cell = %+v want zero rune/width", cont)
	}
	if b.Cell(2, 0).Rune != 'x' {
		t.Fatalf("narrow rune not after wide")
	}
}

func TestSetStringClipsAtRightEdge(t *testing.T) {
	b := New(3, 1)
	next := b.SetString(0, 0, "abcdef", Style{})
	if next != 3 {
		t.Fatalf("next x = %d want 3", next)
	}
	if b.Cell(2, 0).Rune != 'c' {
		t.Fatalf("expected clip at width")
	}
}

func TestSetStringSkipsZeroWidth(t *testing.T) {
	b := New(5, 1)
	// 'e' + combining acute accent (U+0301, width 0)
	b.SetString(0, 0, "éx", Style{})
	if b.Cell(0, 0).Rune != 'e' || b.Cell(1, 0).Rune != 'x' {
		t.Fatalf("combining mark not skipped: %q %q", b.Cell(0, 0).Rune, b.Cell(1, 0).Rune)
	}
}

func TestFillClips(t *testing.T) {
	b := New(3, 3)
	b.Fill(Rect{X: 1, Y: 1, W: 10, H: 10}, Cell{Rune: '#'})
	if b.Cell(0, 0).Rune != ' ' {
		t.Fatalf("fill leaked outside rect")
	}
	if b.Cell(1, 1).Rune != '#' || b.Cell(2, 2).Rune != '#' {
		t.Fatalf("fill did not cover rect")
	}
}

func TestCloneIsIndependent(t *testing.T) {
	b := New(2, 2)
	b.SetString(0, 0, "ab", Style{})
	cp := b.Clone()
	cp.Set(0, 0, Cell{Rune: 'Z'})
	if b.Cell(0, 0).Rune != 'a' {
		t.Fatalf("clone mutated original")
	}
	if cp.Cell(1, 0).Rune != 'b' {
		t.Fatalf("clone did not copy content")
	}
}

func TestResizePreservesOverlap(t *testing.T) {
	b := New(2, 2)
	b.SetString(0, 0, "ab", Style{})
	b.SetString(0, 1, "cd", Style{})
	b.Resize(3, 3)
	if w, h := b.Size(); w != 3 || h != 3 {
		t.Fatalf("size after resize = %d,%d", w, h)
	}
	if b.Cell(0, 0).Rune != 'a' || b.Cell(1, 1).Rune != 'd' {
		t.Fatalf("overlap not preserved")
	}
	if b.Cell(2, 2).Rune != ' ' {
		t.Fatalf("new area not blank")
	}
}

func TestString(t *testing.T) {
	b := New(5, 2)
	b.SetString(0, 0, "hi", Style{})
	b.SetString(1, 1, "yo", Style{})
	got := b.String()
	want := "hi\n yo"
	if got != want {
		t.Fatalf("String() = %q want %q", got, want)
	}
}

func TestStringWideRune(t *testing.T) {
	b := New(4, 1)
	b.SetString(0, 0, "世a", Style{})
	// continuation cell is skipped; wide rune printed once.
	if got := b.String(); got != "世a" {
		t.Fatalf("String() = %q want %q", got, "世a")
	}
}

func TestDiffNoChange(t *testing.T) {
	a := New(4, 2)
	a.SetString(0, 0, "test", Style{})
	b := a.Clone()
	if spans := Diff(a, b); len(spans) != 0 {
		t.Fatalf("Diff of identical buffers = %d spans want 0", len(spans))
	}
}

func TestDiffSingleCell(t *testing.T) {
	a := New(4, 2)
	b := a.Clone()
	b.Set(2, 1, Cell{Rune: 'Z'})
	spans := Diff(a, b)
	want := []Span{{X: 2, Y: 1, Cells: []Cell{{Rune: 'Z', Width: 1}}}}
	if !reflect.DeepEqual(spans, want) {
		t.Fatalf("Diff = %+v want %+v", spans, want)
	}
}

func TestDiffCoalescesRun(t *testing.T) {
	a := New(6, 1)
	b := a.Clone()
	b.SetString(1, 0, "abc", Style{})
	spans := Diff(a, b)
	if len(spans) != 1 {
		t.Fatalf("expected one coalesced span, got %d: %+v", len(spans), spans)
	}
	if spans[0].X != 1 || spans[0].Y != 0 || len(spans[0].Cells) != 3 {
		t.Fatalf("span = %+v want x1 y0 len3", spans[0])
	}
}

func TestDiffSeparateRuns(t *testing.T) {
	a := New(7, 1)
	b := a.Clone()
	b.Set(1, 0, Cell{Rune: 'x'})
	b.Set(4, 0, Cell{Rune: 'y'})
	spans := Diff(a, b)
	if len(spans) != 2 {
		t.Fatalf("expected two spans, got %d: %+v", len(spans), spans)
	}
}

func TestDiffSizeMismatchFullRepaint(t *testing.T) {
	a := New(2, 2)
	b := New(3, 1)
	b.SetString(0, 0, "abc", Style{})
	spans := Diff(a, b)
	if len(spans) != 1 || len(spans[0].Cells) != 3 {
		t.Fatalf("expected full repaint of next, got %+v", spans)
	}
}

func TestStyleAttrHas(t *testing.T) {
	a := AttrBold | AttrUnderline
	if !a.Has(AttrBold) || !a.Has(AttrUnderline) {
		t.Fatalf("Has should report set bits")
	}
	if a.Has(AttrItalic) {
		t.Fatalf("Has should not report unset bit")
	}
}

func TestColorConstructors(t *testing.T) {
	if c := Palette(200); c.Kind != ColorPalette || c.Value != 200 {
		t.Fatalf("Palette wrong: %+v", c)
	}
	if c := RGB(0x12, 0x34, 0x56); c.Kind != ColorRGB || c.Value != 0x123456 {
		t.Fatalf("RGB wrong: %+v", c)
	}
	var zero Color
	if zero.Kind != ColorDefault {
		t.Fatalf("zero Color should be default")
	}
}
