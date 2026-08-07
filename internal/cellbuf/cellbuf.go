// Package cellbuf is karya's in-memory terminal screen model: a grid of styled
// cells that every view renders into, plus a minimal diff that turns two frames
// into the smallest set of write operations.
//
// It is the foundation of karya's single-process TUI (DESIGN.md §6.1) and of its
// testing story (§8.1): a Buffer is pure, allocation-light, and deterministic, so
// views render into one and assert the result as a golden snapshot without a
// terminal. The renderer in internal/term consumes Diff to update the screen,
// touching only the cells that actually changed.
package cellbuf

import "strings"

// ColorKind distinguishes how a Color should be interpreted by the renderer.
type ColorKind uint8

const (
	// ColorDefault is the terminal's default foreground/background.
	ColorDefault ColorKind = iota
	// ColorPalette is an indexed color (0–255): the 16 ANSI colors plus the
	// 240-color extended palette.
	ColorPalette
	// ColorRGB is a 24-bit truecolor value packed as 0xRRGGBB.
	ColorRGB
)

// Color is a terminal color. The zero value is the terminal default, so an
// unset Style renders with the user's own colors.
type Color struct {
	Kind  ColorKind
	Value uint32 // palette index (0–255) or packed 0xRRGGBB
}

// Palette returns an indexed color (0–255).
func Palette(i uint8) Color { return Color{Kind: ColorPalette, Value: uint32(i)} }

// RGB returns a 24-bit truecolor.
func RGB(r, g, b uint8) Color {
	return Color{Kind: ColorRGB, Value: uint32(r)<<16 | uint32(g)<<8 | uint32(b)}
}

// Attr is a bitmask of text attributes.
type Attr uint16

// Text attribute bits.
const (
	AttrBold Attr = 1 << iota
	AttrDim
	AttrItalic
	AttrUnderline
	AttrReverse
	AttrStrike
)

// Has reports whether all bits in a are set.
func (a Attr) Has(bits Attr) bool { return a&bits == bits }

// Style is the visual style of a cell. The zero value renders as plain default
// terminal text.
type Style struct {
	FG    Color
	BG    Color
	Attrs Attr
}

// Cell is one character position on the grid. Width is 1 for a normal rune, 2
// for the leading half of a wide (double-width) rune, and 0 for the trailing
// continuation position that a wide rune occupies. A zero Rune is a blank.
type Cell struct {
	Rune  rune
	Width int8
	Style Style
}

// blank is the empty cell used to clear and pad the grid.
var blank = Cell{Rune: ' ', Width: 1}

// Rect is a rectangular region in cell coordinates.
type Rect struct {
	X, Y, W, H int
}

// Buffer is a fixed-size grid of cells in row-major order.
type Buffer struct {
	w, h  int
	cells []Cell
}

// New returns a w×h Buffer filled with blanks. Non-positive dimensions are
// clamped to zero.
func New(w, h int) *Buffer {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	b := &Buffer{w: w, h: h, cells: make([]Cell, w*h)}
	b.Clear()
	return b
}

// Size returns the buffer's width and height.
func (b *Buffer) Size() (w, h int) { return b.w, b.h }

// Bounds returns the buffer's full rectangle.
func (b *Buffer) Bounds() Rect { return Rect{0, 0, b.w, b.h} }

// Resize changes the buffer's dimensions, preserving overlapping content and
// blank-filling any new area. It is a no-op if the size is unchanged.
func (b *Buffer) Resize(w, h int) {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	if w == b.w && h == b.h {
		return
	}
	next := make([]Cell, w*h)
	for i := range next {
		next[i] = blank
	}
	copyW, copyH := min(w, b.w), min(h, b.h)
	for y := 0; y < copyH; y++ {
		for x := 0; x < copyW; x++ {
			next[y*w+x] = b.cells[y*b.w+x]
		}
	}
	b.w, b.h, b.cells = w, h, next
}

// Clear fills the entire buffer with blank cells.
func (b *Buffer) Clear() {
	for i := range b.cells {
		b.cells[i] = blank
	}
}

// contains reports whether (x,y) is inside the buffer.
func (b *Buffer) contains(x, y int) bool {
	return x >= 0 && y >= 0 && x < b.w && y < b.h
}

// Cell returns the cell at (x,y), or a blank cell if out of bounds.
func (b *Buffer) Cell(x, y int) Cell {
	if !b.contains(x, y) {
		return blank
	}
	return b.cells[y*b.w+x]
}

// Set writes a single cell at (x,y). Out-of-bounds writes are ignored. Set does
// not manage wide-rune continuation cells; use SetString for text.
func (b *Buffer) Set(x, y int, c Cell) {
	if !b.contains(x, y) {
		return
	}
	if c.Width == 0 && c.Rune != 0 {
		c.Width = int8(RuneWidth(c.Rune))
	}
	b.cells[y*b.w+x] = c
}

// SetString writes s starting at (x,y) with style st and returns the x position
// just past the last cell written. Wide runes occupy two cells (a lead cell of
// Width 2 followed by a continuation cell of Width 0); zero-width runes are
// skipped. Writing stops at the right edge of the buffer.
func (b *Buffer) SetString(x, y int, s string, st Style) int {
	if y < 0 || y >= b.h {
		return x
	}
	for _, r := range s {
		w := RuneWidth(r)
		if w == 0 {
			continue
		}
		if x >= b.w {
			break
		}
		if x >= 0 {
			b.cells[y*b.w+x] = Cell{Rune: r, Width: int8(w), Style: st}
			if w == 2 && x+1 < b.w {
				b.cells[y*b.w+x+1] = Cell{Rune: 0, Width: 0, Style: st}
			}
		}
		x += w
	}
	return x
}

// Fill sets every cell within r (clipped to the buffer) to c.
func (b *Buffer) Fill(r Rect, c Cell) {
	if c.Width == 0 && c.Rune != 0 {
		c.Width = int8(RuneWidth(c.Rune))
	}
	x0, y0 := max(r.X, 0), max(r.Y, 0)
	x1, y1 := min(r.X+r.W, b.w), min(r.Y+r.H, b.h)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			b.cells[y*b.w+x] = c
		}
	}
}

// Box draws a single-line border around r with an optional title, styled for
// focus (bold cyan when focused, dim otherwise), and returns the inner content
// rectangle. It is the shared pane-frame primitive used by the IDE window
// manager and by multi-pane views such as the git panel. If r is too small for
// a border it returns r unchanged.
func Box(buf *Buffer, r Rect, title string, focused bool) Rect {
	if r.W < 2 || r.H < 2 {
		return r
	}
	st := Style{}
	if focused {
		st.Attrs |= AttrBold
		st.FG = Palette(6) // cyan for the active pane
	} else {
		st.FG = Palette(8) // dim for inactive
	}
	left, right := r.X, r.X+r.W-1
	top, bottom := r.Y, r.Y+r.H-1

	set := func(x, y int, ru rune) { buf.Set(x, y, Cell{Rune: ru, Width: 1, Style: st}) }
	set(left, top, '┌')
	set(right, top, '┐')
	set(left, bottom, '└')
	set(right, bottom, '┘')
	for x := left + 1; x < right; x++ {
		set(x, top, '─')
		set(x, bottom, '─')
	}
	for y := top + 1; y < bottom; y++ {
		set(left, y, '│')
		set(right, y, '│')
	}
	if title != "" && r.W > 4 {
		label := " " + title + " "
		if len(label) > r.W-2 {
			label = label[:r.W-2]
		}
		buf.SetString(left+2, top, label, st)
	}
	return Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H - 2}
}

// Clone returns a deep copy of the buffer.
func (b *Buffer) Clone() *Buffer {
	cp := &Buffer{w: b.w, h: b.h, cells: make([]Cell, len(b.cells))}
	copy(cp.cells, b.cells)
	return cp
}

// String renders the buffer as plain text (runes only, styles stripped),
// trailing blanks trimmed per line. It is the substrate for golden snapshots.
func (b *Buffer) String() string {
	var sb strings.Builder
	for y := 0; y < b.h; y++ {
		line := make([]rune, 0, b.w)
		for x := 0; x < b.w; x++ {
			c := b.cells[y*b.w+x]
			if c.Width == 0 {
				continue // continuation half of a wide rune
			}
			if c.Rune == 0 {
				line = append(line, ' ')
			} else {
				line = append(line, c.Rune)
			}
		}
		// trim trailing spaces
		end := len(line)
		for end > 0 && line[end-1] == ' ' {
			end--
		}
		sb.WriteString(string(line[:end]))
		if y < b.h-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// Span is a run of contiguous cells on one row that changed between two frames.
// The renderer moves the cursor to (X,Y) and writes Cells left to right.
type Span struct {
	X, Y  int
	Cells []Cell
}

// Diff returns the minimal set of spans needed to turn prev into next. Both
// buffers must be the same size; if they differ, Diff returns a full repaint of
// next. Rows with no changes produce no spans; within a changed row, runs of
// changed cells are coalesced into a single span.
func Diff(prev, next *Buffer) []Span {
	if prev == nil || prev.w != next.w || prev.h != next.h {
		return fullRepaint(next)
	}
	var spans []Span
	for y := 0; y < next.h; y++ {
		x := 0
		for x < next.w {
			if prev.cells[y*next.w+x] == next.cells[y*next.w+x] {
				x++
				continue
			}
			start := x
			run := []Cell{}
			for x < next.w && prev.cells[y*next.w+x] != next.cells[y*next.w+x] {
				run = append(run, next.cells[y*next.w+x])
				x++
			}
			spans = append(spans, Span{X: start, Y: y, Cells: run})
		}
	}
	return spans
}

// fullRepaint returns one span per row covering the whole buffer.
func fullRepaint(b *Buffer) []Span {
	var spans []Span
	for y := 0; y < b.h; y++ {
		run := make([]Cell, b.w)
		copy(run, b.cells[y*b.w:(y+1)*b.w])
		spans = append(spans, Span{X: 0, Y: y, Cells: run})
	}
	return spans
}
