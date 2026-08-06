// Package gateview is karya's gate inbox — the list of tasks waiting on a human
// gate, so multi-task parallelism never buries an approval (DESIGN.md §6). It is
// a thin view over an injected loader; selecting a task opens its review (where
// the human approves or rejects).
package gateview

import (
	"fmt"

	"github.com/drjzlyan/karya/internal/cellbuf"
	"github.com/drjzlyan/karya/internal/term"
)

// Item is one pending gate.
type Item struct {
	ID    string
	State string
	Gate  string
	Title string
}

// Inbox lists tasks awaiting a gate.
type Inbox struct {
	load   func() []Item
	items  []Item
	sel    int
	open   string // set to the selected id when the user asks to open its review
	closed bool
}

// New builds an inbox using load to (re)fetch pending gates.
func New(load func() []Item) *Inbox {
	b := &Inbox{load: load}
	b.refresh()
	return b
}

// Done reports whether the inbox asked to close.
func (b *Inbox) Done() bool { return b.closed }

// OpenRequest returns the id whose review the user asked to open (once), or "".
func (b *Inbox) OpenRequest() string {
	id := b.open
	b.open = ""
	return id
}

func (b *Inbox) refresh() {
	if b.load != nil {
		b.items = b.load()
	}
	if b.sel >= len(b.items) {
		b.sel = max(0, len(b.items)-1)
	}
}

// HandleKey processes a forwarded key.
func (b *Inbox) HandleKey(k term.Key) {
	switch {
	case k == term.RuneKey('j') || k == term.Named(term.SymDown):
		b.move(1)
	case k == term.RuneKey('k') || k == term.Named(term.SymUp):
		b.move(-1)
	case k == term.RuneKey('r'):
		b.refresh()
	case k == term.Named(term.SymEnter) || k == term.RuneKey('o'):
		if b.sel >= 0 && b.sel < len(b.items) {
			b.open = b.items[b.sel].ID
		}
	case k == term.RuneKey('q') || k == term.Named(term.SymEsc):
		b.closed = true
	}
}

func (b *Inbox) move(delta int) {
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

// View renders the inbox. It satisfies layout.PaneContent.
func (b *Inbox) View(buf *cellbuf.Buffer, r cellbuf.Rect, focused bool) {
	if r.W < 4 || r.H < 2 {
		return
	}
	buf.SetString(r.X, r.Y, fit(fmt.Sprintf("  Gate inbox (%d)", len(b.items)), r.W),
		cellbuf.Style{Attrs: cellbuf.AttrBold})
	bottomY := r.Y + r.H - 1
	st := cellbuf.Style{Attrs: cellbuf.AttrReverse}
	buf.Fill(cellbuf.Rect{X: r.X, Y: bottomY, W: r.W, H: 1}, cellbuf.Cell{Rune: ' ', Width: 1, Style: st})
	buf.SetString(r.X, bottomY, fit("j/k move · Enter review · r refresh · q close", r.W), st)

	listY := r.Y + 1
	listH := bottomY - listY
	if len(b.items) == 0 {
		buf.SetString(r.X+2, listY, fit("nothing awaiting a gate", r.W-2), cellbuf.Style{FG: cellbuf.Palette(8)})
		return
	}
	for i, it := range b.items {
		if i >= listH {
			break
		}
		rowStyle := cellbuf.Style{}
		if i == b.sel {
			rowStyle.Attrs |= cellbuf.AttrReverse
		}
		buf.SetString(r.X, listY+i, fit(fmt.Sprintf("gate:%-7s %-12s %s  %s", it.Gate, it.State, it.ID, it.Title), r.W), rowStyle)
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
