// Package taskview is karya's task board — the in-IDE list of human-in-the-loop
// tasks with their gate state (DESIGN.md §6). It is a thin view: task data is
// supplied by an injected loader (the CLI wires it to internal/task), so the
// board is decoupled from the task store and hermetically testable.
package taskview

import (
	"fmt"

	"github.com/drjzlyan/karya/internal/cellbuf"
	"github.com/drjzlyan/karya/internal/term"
)

// Item is one task row on the board.
type Item struct {
	ID    string
	State string
	Title string
}

// Board is the task board view.
type Board struct {
	load      func() []Item
	items     []Item
	sel       int
	status    string
	reviewReq string // set to a task id when the user asks to review it
	agentReq  string // set to a task id when the user asks to run an agent in it
	closed    bool
}

// New builds a board using load to (re)fetch tasks, and loads them immediately.
func New(load func() []Item) *Board {
	b := &Board{load: load}
	b.refresh()
	return b
}

// Done reports whether the board asked to close.
func (b *Board) Done() bool { return b.closed }

// Selected returns the currently highlighted task id, or "" if none.
func (b *Board) Selected() string {
	if b.sel < 0 || b.sel >= len(b.items) {
		return ""
	}
	return b.items[b.sel].ID
}

func (b *Board) refresh() {
	if b.load != nil {
		b.items = b.load()
	}
	if b.sel >= len(b.items) {
		b.sel = max(0, len(b.items)-1)
	}
}

// HandleKey processes a forwarded key.
func (b *Board) HandleKey(k term.Key) {
	switch {
	case k == term.RuneKey('j') || k == term.Named(term.SymDown):
		b.move(1)
	case k == term.RuneKey('k') || k == term.Named(term.SymUp):
		b.move(-1)
	case k == term.RuneKey('r'):
		b.refresh()
	case k == term.Named(term.SymEnter):
		b.reviewReq = b.Selected()
	case k == term.RuneKey('a'):
		b.agentReq = b.Selected()
	case k == term.RuneKey('q') || k == term.Named(term.SymEsc):
		b.closed = true
	}
}

// ReviewRequest returns (once) the task id the user asked to review, or "".
func (b *Board) ReviewRequest() string {
	id := b.reviewReq
	b.reviewReq = ""
	return id
}

// AgentRequest returns (once) the task id the user asked to run an agent in.
func (b *Board) AgentRequest() string {
	id := b.agentReq
	b.agentReq = ""
	return id
}

func (b *Board) move(delta int) {
	if len(b.items) == 0 {
		return
	}
	b.sel += delta
	if b.sel < 0 {
		b.sel = 0
	}
	if b.sel >= len(b.items) {
		b.sel = len(b.items) - 1
	}
}

// View renders the board into rect. It satisfies layout.PaneContent.
func (b *Board) View(buf *cellbuf.Buffer, r cellbuf.Rect, focused bool) {
	if r.W < 4 || r.H < 2 {
		return
	}
	buf.SetString(r.X, r.Y, fit(fmt.Sprintf("  Tasks (%d)", len(b.items)), r.W),
		cellbuf.Style{Attrs: cellbuf.AttrBold})

	bottomY := r.Y + r.H - 1
	b.drawBottom(buf, r, bottomY)

	listY := r.Y + 1
	listH := bottomY - listY
	if len(b.items) == 0 {
		buf.SetString(r.X+2, listY, fit("no tasks — create one with `karya task new <slug>`", r.W-2),
			cellbuf.Style{FG: cellbuf.Palette(8)})
		return
	}
	for i, it := range b.items {
		if i >= listH {
			break
		}
		y := listY + i
		rowStyle := cellbuf.Style{}
		if i == b.sel {
			rowStyle.Attrs |= cellbuf.AttrReverse
		}
		badge := fmt.Sprintf("%-12s", it.State)
		buf.SetString(r.X, y, badge, stateStyle(it.State, rowStyle))
		line := fmt.Sprintf("%s  %s", it.ID, it.Title)
		buf.SetString(r.X+13, y, fit(line, r.W-13), rowStyle)
	}
}

func (b *Board) drawBottom(buf *cellbuf.Buffer, r cellbuf.Rect, y int) {
	st := cellbuf.Style{Attrs: cellbuf.AttrReverse}
	buf.Fill(cellbuf.Rect{X: r.X, Y: y, W: r.W, H: 1}, cellbuf.Cell{Rune: ' ', Width: 1, Style: st})
	text := b.status
	if text == "" {
		text = "j/k move · Enter review · a agent · r refresh · q close"
	}
	buf.SetString(r.X, y, fit(text, r.W), st)
}

// stateStyle colors a state badge, preserving a reverse (selected) background.
func stateStyle(state string, base cellbuf.Style) cellbuf.Style {
	base.FG = stateColor(state)
	return base
}

func stateColor(state string) cellbuf.Color {
	switch state {
	case "draft":
		return cellbuf.Palette(8) // gray
	case "planned", "approved":
		return cellbuf.Palette(3) // yellow
	case "implementing":
		return cellbuf.Palette(4) // blue
	case "verifying":
		return cellbuf.Palette(5) // magenta
	case "merging":
		return cellbuf.Palette(6) // cyan
	case "done":
		return cellbuf.Palette(2) // green
	case "abandoned":
		return cellbuf.Palette(1) // red
	default:
		return cellbuf.Color{}
	}
}

func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if len(s) > w {
		return s[:w]
	}
	return s
}
