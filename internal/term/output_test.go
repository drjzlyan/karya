package term

import (
	"bytes"
	"strings"
	"testing"

	"github.com/drjzlyan/karya/internal/cellbuf"
)

func newTestOutput(caps Caps) (*Output, *bytes.Buffer) {
	var buf bytes.Buffer
	return NewOutput(&buf, caps), &buf
}

func TestOutputControlSequences(t *testing.T) {
	o, buf := newTestOutput(Caps{Palette: 256})
	o.HideCursor()
	o.EnterAltScreen()
	o.ClearScreen()
	o.MoveTo(3, 1)
	if err := o.Flush(); err != nil {
		t.Fatal(err)
	}
	want := "\x1b[?25l\x1b[?1049h\x1b[2J\x1b[H\x1b[2;4H"
	if buf.String() != want {
		t.Fatalf("got %q want %q", buf.String(), want)
	}
}

func TestOutputRenderPlainText(t *testing.T) {
	o, buf := newTestOutput(Caps{Palette: 256})
	spans := []cellbuf.Span{{X: 0, Y: 0, Cells: []cellbuf.Cell{
		{Rune: 'h', Width: 1}, {Rune: 'i', Width: 1},
	}}}
	o.Render(spans)
	o.Flush()
	// Move to (0,0), reset style once (first cell), then "hi".
	want := "\x1b[1;1H\x1b[0mhi"
	if buf.String() != want {
		t.Fatalf("got %q want %q", buf.String(), want)
	}
}

func TestOutputStyleChangeEmitsSGROnce(t *testing.T) {
	o, buf := newTestOutput(Caps{Palette: 256})
	bold := cellbuf.Style{Attrs: cellbuf.AttrBold}
	spans := []cellbuf.Span{{X: 0, Y: 0, Cells: []cellbuf.Cell{
		{Rune: 'a', Width: 1, Style: bold},
		{Rune: 'b', Width: 1, Style: bold}, // same style: no new SGR
		{Rune: 'c', Width: 1},              // default: new SGR
	}}}
	o.Render(spans)
	o.Flush()
	got := buf.String()
	// exactly two SGR sequences: bold, then reset-to-default
	if n := strings.Count(got, "\x1b[0"); n != 2 {
		t.Fatalf("expected 2 SGR sequences, got %d in %q", n, got)
	}
	if !strings.Contains(got, "\x1b[0;1m") {
		t.Fatalf("missing bold SGR in %q", got)
	}
}

func TestOutputWideRuneSkipsContinuation(t *testing.T) {
	o, buf := newTestOutput(Caps{Palette: 256})
	spans := []cellbuf.Span{{X: 0, Y: 0, Cells: []cellbuf.Cell{
		{Rune: '世', Width: 2},
		{Rune: 0, Width: 0}, // continuation: skipped
		{Rune: 'x', Width: 1},
	}}}
	o.Render(spans)
	o.Flush()
	if !strings.HasSuffix(buf.String(), "世x") {
		t.Fatalf("wide rune render wrong: %q", buf.String())
	}
}

func TestSGRTrueColor(t *testing.T) {
	o, _ := newTestOutput(Caps{Palette: 256, TrueColor: true})
	st := cellbuf.Style{FG: cellbuf.RGB(0x10, 0x20, 0x30)}
	if got := o.sgr(st); got != "\x1b[0;38;2;16;32;48m" {
		t.Fatalf("truecolor sgr = %q", got)
	}
}

func TestSGRRGBDownsampledWithoutTruecolor(t *testing.T) {
	o, _ := newTestOutput(Caps{Palette: 256, TrueColor: false})
	st := cellbuf.Style{FG: cellbuf.RGB(0xff, 0x00, 0x00)}
	got := o.sgr(st)
	// pure red -> cube index 16 + 36*5 = 196
	if got != "\x1b[0;38;5;196m" {
		t.Fatalf("downsampled sgr = %q", got)
	}
}

func TestSGRPaletteBasicColors(t *testing.T) {
	o, _ := newTestOutput(Caps{Palette: 256})
	// index 1 (red) fg -> 31 ; index 4 (blue) bg -> 44
	st := cellbuf.Style{FG: cellbuf.Palette(1), BG: cellbuf.Palette(4)}
	if got := o.sgr(st); got != "\x1b[0;31;44m" {
		t.Fatalf("palette sgr = %q", got)
	}
	// bright: index 9 fg -> 91
	st2 := cellbuf.Style{FG: cellbuf.Palette(9)}
	if got := o.sgr(st2); got != "\x1b[0;91m" {
		t.Fatalf("bright palette sgr = %q", got)
	}
	// extended: index 200 fg -> 38;5;200
	st3 := cellbuf.Style{FG: cellbuf.Palette(200)}
	if got := o.sgr(st3); got != "\x1b[0;38;5;200m" {
		t.Fatalf("extended palette sgr = %q", got)
	}
}

func TestDetectCaps(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	cases := []struct {
		name string
		m    map[string]string
		want Caps
	}{
		{"empty", map[string]string{}, Caps{Palette: 8}},
		{"256", map[string]string{"TERM": "xterm-256color"}, Caps{Palette: 256}},
		{"16", map[string]string{"TERM": "xterm"}, Caps{Palette: 16}},
		{"dumb", map[string]string{"TERM": "dumb"}, Caps{Palette: 8}},
		{"truecolor", map[string]string{"TERM": "xterm-256color", "COLORTERM": "truecolor"}, Caps{Palette: 256, TrueColor: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DetectCaps(env(c.m)); got != c.want {
				t.Fatalf("DetectCaps = %+v want %+v", got, c.want)
			}
		})
	}
}
