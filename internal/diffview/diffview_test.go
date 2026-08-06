package diffview

import (
	"testing"

	"github.com/drjzlyan/karya/internal/cellbuf"
)

const sample = `diff --git a/x.go b/x.go
index 111..222 100644
--- a/x.go
+++ b/x.go
@@ -1,3 +1,3 @@
 context line
-removed line
+added line
`

func TestParseClassifies(t *testing.T) {
	lines := Parse(sample)
	want := []LineKind{
		LineFileHeader, // diff --git
		LineMeta,       // index
		LineFileHeader, // ---
		LineFileHeader, // +++
		LineHunk,       // @@
		LineContext,    // " context"
		LineDel,        // -removed
		LineAdd,        // +added
	}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines want %d", len(lines), len(want))
	}
	for i, w := range want {
		if lines[i].Kind != w {
			t.Fatalf("line %d (%q) kind = %d want %d", i, lines[i].Text, lines[i].Kind, w)
		}
	}
}

func TestParseEmpty(t *testing.T) {
	if lines := Parse(""); lines != nil {
		t.Fatalf("empty diff should yield nil, got %v", lines)
	}
}

func TestRenderStylesAndTruncates(t *testing.T) {
	lines := Parse(sample)
	buf := cellbuf.New(20, 8)
	Render(buf, cellbuf.Rect{X: 0, Y: 0, W: 20, H: 8}, lines, 0)

	// The add line should be green.
	// Row 7 is "+added line".
	cell := buf.Cell(0, 7)
	if cell.Rune != '+' || cell.Style.FG != cellbuf.Palette(2) {
		t.Fatalf("add line not green: %+v", cell)
	}
	// The del line (row 6) should be red.
	if c := buf.Cell(0, 6); c.Rune != '-' || c.Style.FG != cellbuf.Palette(1) {
		t.Fatalf("del line not red: %+v", c)
	}
	// The hunk line (row 4) should be bold.
	if c := buf.Cell(0, 4); !c.Style.Attrs.Has(cellbuf.AttrBold) {
		t.Fatalf("hunk line not bold: %+v", c)
	}
}

func TestRenderScroll(t *testing.T) {
	lines := Parse(sample)
	buf := cellbuf.New(30, 2)
	// scroll to the last two lines (del, add)
	Render(buf, cellbuf.Rect{X: 0, Y: 0, W: 30, H: 2}, lines, 6)
	if buf.Cell(0, 0).Rune != '-' || buf.Cell(0, 1).Rune != '+' {
		t.Fatalf("scroll did not show last two lines:\n%s", buf.String())
	}
}

func TestMaxScroll(t *testing.T) {
	lines := Parse(sample) // 8 lines
	if got := MaxScroll(lines, 3); got != 5 {
		t.Fatalf("MaxScroll = %d want 5", got)
	}
	if got := MaxScroll(lines, 100); got != 0 {
		t.Fatalf("MaxScroll when all fit = %d want 0", got)
	}
}

func TestRenderRespectsRectOffset(t *testing.T) {
	lines := Parse(sample)
	buf := cellbuf.New(30, 10)
	Render(buf, cellbuf.Rect{X: 3, Y: 2, W: 27, H: 8}, lines, 0)
	// First line ("diff --git ...") should start at x=3,y=2.
	if buf.Cell(3, 2).Rune != 'd' {
		t.Fatalf("render did not honor rect offset:\n%s", buf.String())
	}
}
