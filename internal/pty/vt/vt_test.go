package vt

import (
	"testing"

	"github.com/drjzlyan/karya/internal/cellbuf"
)

func write(s *Screen, str string) { _, _ = s.Write([]byte(str)) }

func TestPlainText(t *testing.T) {
	s := New(10, 2)
	write(s, "hello")
	if got := s.String(); got != "hello" {
		t.Fatalf("got %q", got)
	}
	if x, y := s.Cursor(); x != 5 || y != 0 {
		t.Fatalf("cursor = %d,%d want 5,0", x, y)
	}
}

func TestCRLF(t *testing.T) {
	s := New(10, 3)
	write(s, "ab\r\ncd")
	if got := s.String(); got != "ab\ncd" {
		t.Fatalf("got %q", got)
	}
}

func TestBackspaceOverwrite(t *testing.T) {
	s := New(10, 1)
	write(s, "abc\b\bX")
	if got := s.String(); got != "aXc" {
		t.Fatalf("got %q", got)
	}
}

func TestTab(t *testing.T) {
	s := New(20, 1)
	write(s, "a\tb")
	// 'a' at 0, tab to 8, 'b' at 8
	if s.buf.Cell(0, 0).Rune != 'a' || s.buf.Cell(8, 0).Rune != 'b' {
		t.Fatalf("tab stop wrong: %q", s.String())
	}
}

func TestWrapAtRightEdge(t *testing.T) {
	s := New(3, 2)
	write(s, "abcd")
	if got := s.String(); got != "abc\nd" {
		t.Fatalf("wrap wrong: %q", got)
	}
}

func TestScrollUp(t *testing.T) {
	s := New(3, 2)
	write(s, "a\r\nb\r\nc")
	// After three lines on a 2-row screen, first line scrolled off.
	if got := s.String(); got != "b\nc" {
		t.Fatalf("scroll wrong: %q", got)
	}
}

func TestCursorPositionCSI(t *testing.T) {
	s := New(10, 3)
	write(s, "\x1b[2;3HX") // row 2 col 3 (1-based) -> (2,1)
	if s.buf.Cell(2, 1).Rune != 'X' {
		t.Fatalf("CUP wrong: %q", s.String())
	}
}

func TestCursorMovements(t *testing.T) {
	s := New(10, 3)
	write(s, "\x1b[3C\x1b[1BZ") // right 3, down 1 -> (3,1)
	if s.buf.Cell(3, 1).Rune != 'Z' {
		t.Fatalf("cursor move wrong: %q", s.String())
	}
}

func TestEraseLine(t *testing.T) {
	s := New(6, 1)
	write(s, "abcdef")
	write(s, "\x1b[3G") // column 3 (1-based) -> cx=2
	write(s, "\x1b[0K") // erase to end of line
	if got := s.String(); got != "ab" {
		t.Fatalf("erase line wrong: %q", got)
	}
}

func TestEraseDisplayAll(t *testing.T) {
	s := New(4, 2)
	write(s, "ab\r\ncd")
	write(s, "\x1b[2J")
	if got := s.String(); got != "" {
		t.Fatalf("erase display wrong: %q", got)
	}
}

func TestSGRColorsApplied(t *testing.T) {
	s := New(6, 1)
	write(s, "\x1b[1;31mA\x1b[0mB")
	a := s.buf.Cell(0, 0)
	if !a.Style.Attrs.Has(cellbuf.AttrBold) {
		t.Fatalf("A should be bold")
	}
	if a.Style.FG != cellbuf.Palette(1) {
		t.Fatalf("A fg = %+v want palette 1", a.Style.FG)
	}
	b := s.buf.Cell(1, 0)
	if b.Style != (cellbuf.Style{}) {
		t.Fatalf("B should be default style, got %+v", b.Style)
	}
}

func TestSGR256AndRGB(t *testing.T) {
	s := New(6, 1)
	write(s, "\x1b[38;5;200mA")
	if s.buf.Cell(0, 0).Style.FG != cellbuf.Palette(200) {
		t.Fatalf("256 color wrong")
	}
	s2 := New(6, 1)
	write(s2, "\x1b[48;2;10;20;30mB")
	if s2.buf.Cell(0, 0).Style.BG != cellbuf.RGB(10, 20, 30) {
		t.Fatalf("rgb color wrong")
	}
}

func TestUTF8SplitAcrossWrites(t *testing.T) {
	s := New(4, 1)
	b := []byte("世") // 3 bytes
	_, _ = s.Write(b[:2])
	_, _ = s.Write(b[2:])
	if s.buf.Cell(0, 0).Rune != '世' {
		t.Fatalf("split utf8 wrong: %q", s.String())
	}
}

func TestPrivateModeIgnored(t *testing.T) {
	s := New(6, 1)
	write(s, "\x1b[?25lX\x1b[?25h") // hide/show cursor around X: ignored, X printed
	if s.buf.Cell(0, 0).Rune != 'X' {
		t.Fatalf("private mode should be ignored, got %q", s.String())
	}
}

func TestSaveRestoreCursor(t *testing.T) {
	s := New(10, 2)
	write(s, "ab\x1b7cd\x1b8X") // save at (2,0) after ab, move, restore, X at (2,0)
	if s.buf.Cell(2, 0).Rune != 'X' {
		t.Fatalf("save/restore cursor wrong: %q", s.String())
	}
}

func TestResizeClampsCursor(t *testing.T) {
	s := New(10, 5)
	write(s, "\x1b[5;9HX")
	s.Resize(4, 2)
	if x, y := s.Cursor(); x > 3 || y > 1 {
		t.Fatalf("cursor not clamped after resize: %d,%d", x, y)
	}
}
