package finder

import (
	"fmt"

	"github.com/drjzlyan/karya/internal/cellbuf"
	"github.com/drjzlyan/karya/internal/term"
)

// Finder is the fuzzy file-finder view: a query line over a filtered file list.
type Finder struct {
	all      []string
	query    string
	filtered []string
	sel      int
	open     string
	closed   bool
}

// New builds a finder over the given candidate paths.
func New(items []string) *Finder {
	f := &Finder{all: items}
	f.refilter()
	return f
}

// Done reports whether the finder asked to close.
func (f *Finder) Done() bool { return f.closed }

// OpenRequest returns (once) the path the user chose to open, or "".
func (f *Finder) OpenRequest() string {
	p := f.open
	f.open = ""
	return p
}

func (f *Finder) refilter() {
	f.filtered = Filter(f.query, f.all)
	if f.sel >= len(f.filtered) {
		f.sel = max(0, len(f.filtered)-1)
	}
}

// HandleKey processes a forwarded key.
func (f *Finder) HandleKey(k term.Key) {
	switch {
	case k == term.Named(term.SymEsc):
		f.closed = true
	case k == term.Named(term.SymEnter):
		if f.sel >= 0 && f.sel < len(f.filtered) {
			f.open = f.filtered[f.sel]
		}
	case k == term.Named(term.SymDown) || k == term.Ctrl('n'):
		f.move(1)
	case k == term.Named(term.SymUp) || k == term.Ctrl('p'):
		f.move(-1)
	case k == term.Named(term.SymBackspace):
		if n := len(f.query); n > 0 {
			f.query = f.query[:n-1]
			f.sel = 0
			f.refilter()
		}
	case k.Sym == term.SymRune && k.Mod == 0:
		f.query += string(k.Rune)
		f.sel = 0
		f.refilter()
	}
}

func (f *Finder) move(delta int) {
	if len(f.filtered) == 0 {
		return
	}
	f.sel += delta
	if f.sel < 0 {
		f.sel = 0
	}
	if f.sel >= len(f.filtered) {
		f.sel = len(f.filtered) - 1
	}
}

// View renders the finder. It satisfies layout.PaneContent.
func (f *Finder) View(buf *cellbuf.Buffer, r cellbuf.Rect, focused bool) {
	if r.W < 4 || r.H < 3 {
		return
	}
	// Query line.
	buf.SetString(r.X, r.Y, fit(fmt.Sprintf("> %s_", f.query), r.W), cellbuf.Style{Attrs: cellbuf.AttrBold})
	// Footer.
	bottomY := r.Y + r.H - 1
	st := cellbuf.Style{Attrs: cellbuf.AttrReverse}
	buf.Fill(cellbuf.Rect{X: r.X, Y: bottomY, W: r.W, H: 1}, cellbuf.Cell{Rune: ' ', Width: 1, Style: st})
	buf.SetString(r.X, bottomY, fit(fmt.Sprintf("find file · %d matches · Enter open · Esc close", len(f.filtered)), r.W), st)
	// Results.
	listY := r.Y + 1
	listH := bottomY - listY
	for i := 0; i < listH && i < len(f.filtered); i++ {
		rowStyle := cellbuf.Style{}
		if i == f.sel {
			rowStyle.Attrs |= cellbuf.AttrReverse
		}
		buf.SetString(r.X, listY+i, fit(f.filtered[i], r.W), rowStyle)
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
