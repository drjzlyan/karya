// Package vt is a minimal terminal (VT100/xterm) emulator: it interprets the
// byte stream a child process writes to its pty into a cellbuf screen that a
// karya pane can blit (DESIGN.md §6.1). It is deliberately small — enough to
// render shells and agent CLIs faithfully — and pure (bytes in, screen out), so
// it is unit-testable without any real process.
package vt

import (
	"strings"
	"unicode/utf8"

	"github.com/drjzlyan/karya/internal/cellbuf"
)

// tabWidth is the fixed tab stop interval.
const tabWidth = 8

type parseState uint8

const (
	stateGround parseState = iota
	stateEsc
	stateCSI
)

// Screen is an in-memory terminal screen updated by writing child output to it.
type Screen struct {
	buf      *cellbuf.Buffer
	w, h     int
	cx, cy   int
	style    cellbuf.Style
	state    parseState
	params   []byte
	pending  []byte // incomplete UTF-8 carried across writes
	private  bool   // CSI '?' private-mode marker (ignored actions)
	wrapNext bool   // deferred wrap: cursor is parked at the last column
	savedX   int
	savedY   int
}

// New returns a w×h blank Screen.
func New(w, h int) *Screen {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return &Screen{buf: cellbuf.New(w, h), w: w, h: h}
}

// Resize changes the screen size, clamping the cursor into range.
func (s *Screen) Resize(w, h int) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	s.buf.Resize(w, h)
	s.w, s.h = w, h
	s.cx = min(s.cx, w-1)
	s.cy = min(s.cy, h-1)
}

// Buffer returns the current screen buffer (do not mutate).
func (s *Screen) Buffer() *cellbuf.Buffer { return s.buf }

// Cursor returns the cursor position.
func (s *Screen) Cursor() (x, y int) { return s.cx, s.cy }

// String renders the screen as text with trailing blank lines trimmed, for
// snapshot tests.
func (s *Screen) String() string {
	lines := strings.Split(s.buf.String(), "\n")
	end := len(lines)
	for end > 0 && lines[end-1] == "" {
		end--
	}
	return strings.Join(lines[:end], "\n")
}

// Write feeds child output into the emulator.
func (s *Screen) Write(p []byte) (int, error) {
	data := p
	if len(s.pending) > 0 {
		data = append(s.pending, p...)
		s.pending = nil
	}
	i := 0
	for i < len(data) {
		b := data[i]
		switch s.state {
		case stateGround:
			if b == 0x1b {
				s.state = stateEsc
				i++
				continue
			}
			if b < 0x20 {
				s.control(b)
				i++
				continue
			}
			// printable: may be multibyte UTF-8
			r, size := utf8.DecodeRune(data[i:])
			if r == utf8.RuneError && size == 1 && !utf8.FullRune(data[i:]) {
				// incomplete rune at the end: stash and wait
				s.pending = append(s.pending, data[i:]...)
				return len(p), nil
			}
			s.put(r)
			i += size
		case stateEsc:
			switch b {
			case '[':
				s.state = stateCSI
				s.params = s.params[:0]
				s.private = false
			case '7':
				s.savedX, s.savedY = s.cx, s.cy
				s.state = stateGround
			case '8':
				s.cx, s.cy = s.savedX, s.savedY
				s.state = stateGround
			default:
				// Unhandled two-byte escape; ignore.
				s.state = stateGround
			}
			i++
		case stateCSI:
			if b == '?' && len(s.params) == 0 {
				s.private = true
				i++
				continue
			}
			if (b >= '0' && b <= '9') || b == ';' {
				s.params = append(s.params, b)
				i++
				continue
			}
			s.csi(b)
			s.state = stateGround
			i++
		}
	}
	return len(p), nil
}

// control handles a C0 control byte in the ground state.
func (s *Screen) control(b byte) {
	switch b {
	case '\r':
		s.cx = 0
		s.wrapNext = false
	case '\n', 0x0b, 0x0c: // LF, VT, FF
		s.lineFeed()
		s.wrapNext = false
	case '\t':
		s.cx = ((s.cx / tabWidth) + 1) * tabWidth
		if s.cx >= s.w {
			s.cx = s.w - 1
		}
		s.wrapNext = false
	case '\b':
		if s.cx > 0 {
			s.cx--
		}
		s.wrapNext = false
	}
}

// put writes a printable rune at the cursor, wrapping and scrolling as needed.
// It uses deferred wrapping: after a char lands in the last column the cursor
// stays there (wrapNext set) and only wraps when the next printable char comes,
// matching how real terminals behave.
func (s *Screen) put(r rune) {
	w := cellbuf.RuneWidth(r)
	if w == 0 {
		return
	}
	if s.wrapNext {
		s.cx = 0
		s.lineFeed()
		s.wrapNext = false
	}
	if s.cx+w > s.w {
		// A wide char with no room at the edge: wrap first.
		s.cx = 0
		s.lineFeed()
	}
	s.buf.Set(s.cx, s.cy, cellbuf.Cell{Rune: r, Width: int8(w), Style: s.style})
	if w == 2 && s.cx+1 < s.w {
		s.buf.Set(s.cx+1, s.cy, cellbuf.Cell{Rune: 0, Width: 0, Style: s.style})
	}
	s.cx += w
	if s.cx >= s.w {
		s.cx = s.w - 1
		s.wrapNext = true
	}
}

// lineFeed moves down one line, scrolling the screen if at the bottom.
func (s *Screen) lineFeed() {
	if s.cy < s.h-1 {
		s.cy++
		return
	}
	s.scrollUp()
}

// scrollUp shifts every row up by one and blanks the last row.
func (s *Screen) scrollUp() {
	for y := 0; y < s.h-1; y++ {
		for x := 0; x < s.w; x++ {
			s.buf.Set(x, y, s.buf.Cell(x, y+1))
		}
	}
	blank := cellbuf.Cell{Rune: ' ', Width: 1}
	for x := 0; x < s.w; x++ {
		s.buf.Set(x, s.h-1, blank)
	}
}
