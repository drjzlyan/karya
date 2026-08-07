package term

import (
	"bytes"
	"io"
	"strconv"

	"github.com/drjzlyan/karya/internal/cellbuf"
)

// Output buffers ANSI control and text and flushes it to the underlying writer
// in one write per frame. It tracks the last applied style so it emits SGR only
// when the style changes. Writing to a bytes.Buffer makes output byte-exactly
// testable.
type Output struct {
	w    io.Writer
	caps Caps
	buf  bytes.Buffer

	haveStyle bool
	curStyle  cellbuf.Style
}

// NewOutput returns an Output writing to w with the given capabilities.
func NewOutput(w io.Writer, caps Caps) *Output {
	return &Output{w: w, caps: caps}
}

// raw appends a raw string to the frame buffer.
func (o *Output) raw(s string) { o.buf.WriteString(s) }

// HideCursor / ShowCursor toggle the terminal cursor.
func (o *Output) HideCursor() { o.raw("\x1b[?25l") }
func (o *Output) ShowCursor() { o.raw("\x1b[?25h") }

// EnterAltScreen / ExitAltScreen switch to and from the alternate screen buffer.
func (o *Output) EnterAltScreen() { o.raw("\x1b[?1049h") }
func (o *Output) ExitAltScreen()  { o.raw("\x1b[?1049l") }

// EnableMouse / DisableMouse toggle SGR mouse reporting.
func (o *Output) EnableMouse()  { o.raw("\x1b[?1000h\x1b[?1002h\x1b[?1006h") }
func (o *Output) DisableMouse() { o.raw("\x1b[?1006l\x1b[?1002l\x1b[?1000l") }

// EnableBracketedPaste / DisableBracketedPaste toggle bracketed paste.
func (o *Output) EnableBracketedPaste()  { o.raw("\x1b[?2004h") }
func (o *Output) DisableBracketedPaste() { o.raw("\x1b[?2004l") }

// ClearScreen clears the whole screen and homes the cursor.
func (o *Output) ClearScreen() { o.raw("\x1b[2J\x1b[H") }

// MoveTo positions the cursor at zero-based (x,y).
func (o *Output) MoveTo(x, y int) {
	o.raw("\x1b[")
	o.raw(strconv.Itoa(y + 1))
	o.raw(";")
	o.raw(strconv.Itoa(x + 1))
	o.raw("H")
}

// Reset clears any active SGR styling.
func (o *Output) Reset() {
	o.raw("\x1b[0m")
	o.haveStyle = false
	o.curStyle = cellbuf.Style{}
}

// Render writes the given diff spans: it moves to each span's origin and writes
// its cells, changing SGR only when a cell's style differs from the last one.
func (o *Output) Render(spans []cellbuf.Span) {
	for _, sp := range spans {
		o.MoveTo(sp.X, sp.Y)
		for _, c := range sp.Cells {
			if c.Width == 0 {
				continue // continuation half of a wide rune
			}
			o.applyStyle(c.Style)
			if c.Rune == 0 {
				o.buf.WriteByte(' ')
			} else {
				o.buf.WriteRune(c.Rune)
			}
		}
	}
}

// applyStyle emits an SGR sequence if style differs from the current one.
func (o *Output) applyStyle(style cellbuf.Style) {
	if o.haveStyle && style == o.curStyle {
		return
	}
	o.raw(o.sgr(style))
	o.haveStyle = true
	o.curStyle = style
}

// Flush writes the buffered frame to the underlying writer and resets the
// buffer.
func (o *Output) Flush() error {
	if o.buf.Len() == 0 {
		return nil
	}
	_, err := o.w.Write(o.buf.Bytes())
	o.buf.Reset()
	return err
}

// sgr builds a full SGR sequence for style (always starting from a reset so the
// result is self-contained).
func (o *Output) sgr(style cellbuf.Style) string {
	codes := []string{"0"}
	a := style.Attrs
	if a.Has(cellbuf.AttrBold) {
		codes = append(codes, "1")
	}
	if a.Has(cellbuf.AttrDim) {
		codes = append(codes, "2")
	}
	if a.Has(cellbuf.AttrItalic) {
		codes = append(codes, "3")
	}
	if a.Has(cellbuf.AttrUnderline) {
		codes = append(codes, "4")
	}
	if a.Has(cellbuf.AttrReverse) {
		codes = append(codes, "7")
	}
	if a.Has(cellbuf.AttrStrike) {
		codes = append(codes, "9")
	}
	codes = append(codes, o.colorCodes(style.FG, false)...)
	codes = append(codes, o.colorCodes(style.BG, true)...)

	var b bytes.Buffer
	b.WriteString("\x1b[")
	for i, c := range codes {
		if i > 0 {
			b.WriteByte(';')
		}
		b.WriteString(c)
	}
	b.WriteByte('m')
	return b.String()
}

// colorCodes returns the SGR parameters for a foreground (bg=false) or
// background (bg=true) color, respecting the terminal's color depth.
func (o *Output) colorCodes(c cellbuf.Color, bg bool) []string {
	switch c.Kind {
	case cellbuf.ColorDefault:
		return nil
	case cellbuf.ColorPalette:
		return paletteCodes(uint8(c.Value), bg)
	case cellbuf.ColorRGB:
		r := uint8(c.Value >> 16)
		g := uint8(c.Value >> 8)
		b := uint8(c.Value)
		if o.caps.TrueColor {
			base := "38;2"
			if bg {
				base = "48;2"
			}
			return []string{base, strconv.Itoa(int(r)), strconv.Itoa(int(g)), strconv.Itoa(int(b))}
		}
		return paletteCodes(rgbTo256(r, g, b), bg)
	}
	return nil
}

// paletteCodes returns SGR params for an indexed color, using the compact 3x/4x
// (and 9x/10x) forms for the first 16 colors.
func paletteCodes(idx uint8, bg bool) []string {
	switch {
	case idx < 8:
		base := 30
		if bg {
			base = 40
		}
		return []string{strconv.Itoa(base + int(idx))}
	case idx < 16:
		base := 90
		if bg {
			base = 100
		}
		return []string{strconv.Itoa(base + int(idx-8))}
	default:
		prefix := "38;5"
		if bg {
			prefix = "48;5"
		}
		return []string{prefix, strconv.Itoa(int(idx))}
	}
}

// rgbTo256 downsamples a truecolor value to the nearest 256-color palette index
// using the 6×6×6 color cube.
func rgbTo256(r, g, b uint8) uint8 {
	q := func(v uint8) int { return int(v) * 6 / 256 }
	return uint8(16 + 36*q(r) + 6*q(g) + q(b))
}
