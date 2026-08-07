package searchview

import (
	"fmt"

	"github.com/drjzlyan/karya/internal/cellbuf"
	"github.com/drjzlyan/karya/internal/term"
)

type mode uint8

const (
	modeInput mode = iota
	modeResults
)

// Search is the project-search view: a query line and a results list. Enter in
// the query runs the search; Enter on a result opens it in the editor.
type Search struct {
	dir     string
	search  Searcher
	query   string
	results []Match
	sel     int
	mode    mode
	open    *Match
	closed  bool
}

// New builds a search view for dir using searcher.
func New(dir string, searcher Searcher) *Search {
	return &Search{dir: dir, search: searcher}
}

// Done reports whether the view asked to close.
func (s *Search) Done() bool { return s.closed }

// OpenRequest returns (once) the match the user chose to open, or nil.
func (s *Search) OpenRequest() *Match {
	m := s.open
	s.open = nil
	return m
}

// HandleKey processes a forwarded key.
func (s *Search) HandleKey(k term.Key) {
	if s.mode == modeResults {
		s.handleResults(k)
		return
	}
	switch {
	case k == term.Named(term.SymEsc):
		s.closed = true
	case k == term.Named(term.SymEnter):
		s.results = s.search(s.dir, s.query)
		s.sel = 0
		if len(s.results) > 0 {
			s.mode = modeResults
		}
	case k == term.Named(term.SymBackspace):
		if n := len(s.query); n > 0 {
			s.query = s.query[:n-1]
		}
	case k.Sym == term.SymRune && k.Mod == 0:
		s.query += string(k.Rune)
	}
}

func (s *Search) handleResults(k term.Key) {
	switch {
	case k == term.Named(term.SymDown) || k == term.Ctrl('n') || k == term.RuneKey('j'):
		s.move(1)
	case k == term.Named(term.SymUp) || k == term.Ctrl('p') || k == term.RuneKey('k'):
		s.move(-1)
	case k == term.Named(term.SymEnter):
		if s.sel >= 0 && s.sel < len(s.results) {
			m := s.results[s.sel]
			s.open = &m
		}
	case k == term.RuneKey('i') || k == term.RuneKey('/'):
		s.mode = modeInput
	case k == term.Named(term.SymEsc):
		s.mode = modeInput
	}
}

func (s *Search) move(delta int) {
	if len(s.results) == 0 {
		return
	}
	s.sel += delta
	if s.sel < 0 {
		s.sel = 0
	}
	if s.sel >= len(s.results) {
		s.sel = len(s.results) - 1
	}
}

// View renders the search view. It satisfies layout.PaneContent.
func (s *Search) View(buf *cellbuf.Buffer, r cellbuf.Rect, focused bool) {
	if r.W < 4 || r.H < 3 {
		return
	}
	caret := ""
	if s.mode == modeInput {
		caret = "_"
	}
	buf.SetString(r.X, r.Y, fit("search: "+s.query+caret, r.W), cellbuf.Style{Attrs: cellbuf.AttrBold})

	bottomY := r.Y + r.H - 1
	st := cellbuf.Style{Attrs: cellbuf.AttrReverse}
	buf.Fill(cellbuf.Rect{X: r.X, Y: bottomY, W: r.W, H: 1}, cellbuf.Cell{Rune: ' ', Width: 1, Style: st})
	footer := "type a query · Enter search · Esc close"
	if s.mode == modeResults {
		footer = fmt.Sprintf("%d matches · j/k move · Enter open · i edit query · Esc back", len(s.results))
	}
	buf.SetString(r.X, bottomY, fit(footer, r.W), st)

	listY := r.Y + 1
	listH := bottomY - listY
	for i := 0; i < listH && i < len(s.results); i++ {
		m := s.results[i]
		rowStyle := cellbuf.Style{}
		if i == s.sel && s.mode == modeResults {
			rowStyle.Attrs |= cellbuf.AttrReverse
		}
		loc := fmt.Sprintf("%s:%d: ", m.File, m.Line)
		buf.SetString(r.X, listY+i, fit(loc+m.Text, r.W), rowStyle)
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
