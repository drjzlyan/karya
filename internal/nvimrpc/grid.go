package nvimrpc

import (
	"github.com/drjzlyan/karya/internal/cellbuf"
)

// Grid is the reduced state of Neovim's UI: the screen contents (as a cellbuf
// Buffer), the cursor position, the highlight table, and the current mode. It is
// updated by applying `redraw` notification batches and is a pure function of
// the events seen — no RPC or terminal involved — so rendering is snapshot
// testable from recorded/synthetic batches (DESIGN.md §6.3, §8.1).
//
// karya attaches with a single global grid (grid id 1); ext_multigrid is
// deferred, so only that grid is modeled.
type Grid struct {
	buf    *cellbuf.Buffer
	cursor struct{ row, col int }
	mode   string

	defFG cellbuf.Color
	defBG cellbuf.Color
	hl    map[int]map[string]any
}

// NewGrid returns a w×h grid.
func NewGrid(w, h int) *Grid {
	return &Grid{buf: cellbuf.New(w, h), hl: map[int]map[string]any{}}
}

// Buffer returns the current screen buffer (do not mutate).
func (g *Grid) Buffer() *cellbuf.Buffer { return g.buf }

// Cursor returns the cursor row and column.
func (g *Grid) Cursor() (row, col int) { return g.cursor.row, g.cursor.col }

// Mode returns the current editor mode name (e.g. "normal", "insert").
func (g *Grid) Mode() string { return g.mode }

// Size returns the grid dimensions.
func (g *Grid) Size() (w, h int) { return g.buf.Size() }

// HandleRedraw applies one `redraw` notification's parameters. params is the
// array of event groups, each [name, inst1, inst2, ...]. It returns true if the
// batch ended with a "flush" (the point at which the caller should repaint).
func (g *Grid) HandleRedraw(params []any) (flushed bool) {
	for _, grp := range params {
		group, ok := grp.([]any)
		if !ok || len(group) == 0 {
			continue
		}
		name, _ := group[0].(string)
		if name == "flush" {
			flushed = true
			continue
		}
		for _, inst := range group[1:] {
			args, ok := inst.([]any)
			if !ok {
				continue
			}
			g.dispatch(name, args)
		}
	}
	return flushed
}

func (g *Grid) dispatch(name string, a []any) {
	switch name {
	case "grid_resize":
		// [grid, width, height]
		if len(a) >= 3 {
			g.buf.Resize(toInt(a[1]), toInt(a[2]))
		}
	case "grid_clear":
		g.buf.Clear()
	case "grid_cursor_goto":
		// [grid, row, col]
		if len(a) >= 3 {
			g.cursor.row, g.cursor.col = toInt(a[1]), toInt(a[2])
		}
	case "default_colors_set":
		// [rgb_fg, rgb_bg, rgb_sp, cterm_fg, cterm_bg]
		if len(a) >= 2 {
			g.defFG = rgbColor(a[0])
			g.defBG = rgbColor(a[1])
		}
	case "hl_attr_define":
		// [id, rgb_attrs, cterm_attrs, info]
		if len(a) >= 2 {
			id := toInt(a[0])
			if m, ok := a[1].(map[string]any); ok {
				g.hl[id] = m
			}
		}
	case "grid_line":
		g.applyLine(a)
	case "grid_scroll":
		g.applyScroll(a)
	case "mode_change":
		// [mode, mode_idx]
		if len(a) >= 1 {
			g.mode, _ = a[0].(string)
		}
	}
}

// applyLine handles ["grid", row, col_start, cells, wrap] where each cell is
// [text, hl_id?, repeat?]; hl_id persists across cells when omitted.
func (g *Grid) applyLine(a []any) {
	if len(a) < 4 {
		return
	}
	row := toInt(a[1])
	col := toInt(a[2])
	cells, ok := a[3].([]any)
	if !ok {
		return
	}
	lastHl := 0
	for _, c := range cells {
		cell, ok := c.([]any)
		if !ok || len(cell) == 0 {
			continue
		}
		text, _ := cell[0].(string)
		if len(cell) >= 2 {
			if cell[1] != nil {
				lastHl = toInt(cell[1])
			}
		}
		repeat := 1
		if len(cell) >= 3 {
			repeat = toInt(cell[2])
		}
		style := g.styleFor(lastHl)
		r := firstRune(text)
		w := cellbuf.RuneWidth(r)
		if w < 1 {
			w = 1
		}
		for i := 0; i < repeat; i++ {
			g.buf.Set(col, row, cellbuf.Cell{Rune: r, Width: int8(w), Style: style})
			if w == 2 {
				g.buf.Set(col+1, row, cellbuf.Cell{Rune: 0, Width: 0, Style: style})
			}
			col += w
		}
	}
}

// applyScroll handles ["grid", top, bot, left, right, rows, cols] by shifting the
// region; newly exposed lines are filled by subsequent grid_line events.
func (g *Grid) applyScroll(a []any) {
	if len(a) < 7 {
		return
	}
	top, bot := toInt(a[1]), toInt(a[2])
	left, right := toInt(a[3]), toInt(a[4])
	rows := toInt(a[5])
	if rows > 0 { // content moves up
		for y := top; y < bot-rows; y++ {
			for x := left; x < right; x++ {
				g.buf.Set(x, y, g.buf.Cell(x, y+rows))
			}
		}
	} else if rows < 0 { // content moves down
		for y := bot - 1; y >= top-rows; y-- {
			for x := left; x < right; x++ {
				g.buf.Set(x, y, g.buf.Cell(x, y+rows))
			}
		}
	}
}

// styleFor builds the cellbuf style for a highlight id, inheriting default
// colors and applying the attribute flags Neovim defined.
func (g *Grid) styleFor(hl int) cellbuf.Style {
	st := cellbuf.Style{FG: g.defFG, BG: g.defBG}
	if hl == 0 {
		return st
	}
	attr := g.hl[hl]
	if attr == nil {
		return st
	}
	if v, ok := attr["foreground"]; ok {
		st.FG = rgbColor(v)
	}
	if v, ok := attr["background"]; ok {
		st.BG = rgbColor(v)
	}
	if truthy(attr["bold"]) {
		st.Attrs |= cellbuf.AttrBold
	}
	if truthy(attr["italic"]) {
		st.Attrs |= cellbuf.AttrItalic
	}
	if truthy(attr["underline"]) || truthy(attr["undercurl"]) || truthy(attr["underdouble"]) {
		st.Attrs |= cellbuf.AttrUnderline
	}
	if truthy(attr["strikethrough"]) {
		st.Attrs |= cellbuf.AttrStrike
	}
	if truthy(attr["reverse"]) {
		st.Attrs |= cellbuf.AttrReverse
	}
	return st
}

// --- conversion helpers (msgpack decodes ints as int64) ---

func toInt(v any) int {
	switch x := v.(type) {
	case int64:
		return int(x)
	case int:
		return x
	case float64:
		return int(x)
	}
	return 0
}

func truthy(v any) bool {
	b, _ := v.(bool)
	return b
}

// rgbColor converts a Neovim packed-RGB integer to a cellbuf color; a negative
// value (Neovim's "default") maps to the terminal default.
func rgbColor(v any) cellbuf.Color {
	n := toInt(v)
	if n < 0 {
		return cellbuf.Color{}
	}
	return cellbuf.RGB(uint8(n>>16), uint8(n>>8), uint8(n))
}

func firstRune(s string) rune {
	for _, r := range s {
		return r
	}
	return ' '
}
