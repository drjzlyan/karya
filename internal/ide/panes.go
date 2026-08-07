package ide

import (
	"os"
	"os/exec"
	"unicode/utf8"

	"github.com/drjzlyan/karya/internal/cellbuf"
	"github.com/drjzlyan/karya/internal/layout"
	"github.com/drjzlyan/karya/internal/pty"
	"github.com/drjzlyan/karya/internal/pty/vt"
	"github.com/drjzlyan/karya/internal/term"
)

// placeholderPane renders a centered label. It is used as a fallback when a
// shell cannot be spawned and for the welcome pane.
type placeholderPane struct {
	title string
	body  string
}

func (p *placeholderPane) View(buf *cellbuf.Buffer, r cellbuf.Rect, focused bool) {
	if r.W <= 0 || r.H <= 0 {
		return
	}
	lines := []string{p.title, "", p.body}
	startY := r.Y + max(0, (r.H-len(lines))/2)
	for i, ln := range lines {
		y := startY + i
		if y >= r.Y+r.H {
			break
		}
		x := r.X + max(0, (r.W-len(ln))/2)
		buf.SetString(x, y, ln, cellbuf.Style{})
	}
}

// shellPane hosts a shell (or agent CLI) on a PTY and renders its screen via the
// vt emulator.
type shellPane struct {
	pty    *pty.PTY
	screen *vt.Screen
	dead   bool
	dir    string
}

// newShellPane spawns cmd on a PTY sized cols×rows.
func newShellPane(cmd *exec.Cmd, dir string, cols, rows int) (*shellPane, error) {
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	p, err := pty.Start(cmd, cols, rows)
	if err != nil {
		return nil, err
	}
	return &shellPane{pty: p, screen: vt.New(cols, rows), dir: dir}, nil
}

// defaultShellPane spawns the user's shell (or /bin/sh) in dir.
func defaultShellPane(dir string, cols, rows int) (*shellPane, error) {
	sh := os.Getenv("SHELL")
	if sh == "" {
		sh = "/bin/sh"
	}
	cmd := exec.Command(sh)
	cmd.Dir = dir
	return newShellPane(cmd, dir, cols, rows)
}

func (s *shellPane) View(buf *cellbuf.Buffer, r cellbuf.Rect, focused bool) {
	src := s.screen.Buffer()
	sw, sh := src.Size()
	for y := 0; y < r.H && y < sh; y++ {
		for x := 0; x < r.W && x < sw; x++ {
			buf.Set(r.X+x, r.Y+y, src.Cell(x, y))
		}
	}
}

// resize matches the PTY and emulator to a new inner size.
func (s *shellPane) resize(cols, rows int) {
	if cols < 1 || rows < 1 {
		return
	}
	s.screen.Resize(cols, rows)
	if s.pty != nil {
		_ = s.pty.Resize(cols, rows)
	}
}

// write forwards input bytes to the shell.
func (s *shellPane) write(b []byte) {
	if s.pty != nil && len(b) > 0 {
		_, _ = s.pty.Write(b)
	}
}

func (s *shellPane) close() {
	if s.pty != nil {
		_ = s.pty.Close()
	}
}

// drawFrame draws a box around r with title, styled by focus, and returns the
// inner content rectangle. It delegates to cellbuf.Box, the shared pane-frame
// primitive.
func drawFrame(buf *cellbuf.Buffer, r cellbuf.Rect, title string, focused bool) cellbuf.Rect {
	return cellbuf.Box(buf, r, title, focused)
}

// encodeKey converts a Key into the byte sequence a terminal application (the
// shell) expects on its input.
func encodeKey(k term.Key) []byte {
	if k.Sym == term.SymRune {
		switch {
		case k.Mod.Has(term.ModCtrl):
			r := k.Rune
			if r == ' ' || r == '@' {
				return []byte{0}
			}
			up := r
			if up >= 'a' && up <= 'z' {
				up -= 'a' - 'A'
			}
			if up >= '@' && up <= '_' {
				return []byte{byte(up & 0x1f)}
			}
			return runeBytes(r)
		case k.Mod.Has(term.ModAlt):
			return append([]byte{0x1b}, runeBytes(k.Rune)...)
		default:
			return runeBytes(k.Rune)
		}
	}
	switch k.Sym {
	case term.SymEnter:
		return []byte{'\r'}
	case term.SymTab:
		return []byte{'\t'}
	case term.SymBackspace:
		return []byte{0x7f}
	case term.SymEsc:
		return []byte{0x1b}
	case term.SymUp:
		return []byte("\x1b[A")
	case term.SymDown:
		return []byte("\x1b[B")
	case term.SymRight:
		return []byte("\x1b[C")
	case term.SymLeft:
		return []byte("\x1b[D")
	case term.SymHome:
		return []byte("\x1b[H")
	case term.SymEnd:
		return []byte("\x1b[F")
	case term.SymInsert:
		return []byte("\x1b[2~")
	case term.SymDelete:
		return []byte("\x1b[3~")
	case term.SymPageUp:
		return []byte("\x1b[5~")
	case term.SymPageDown:
		return []byte("\x1b[6~")
	}
	return nil
}

func runeBytes(r rune) []byte {
	buf := make([]byte, utf8.RuneLen(r))
	utf8.EncodeRune(buf, r)
	return buf
}

// ensure the pane types satisfy the layout contract.
var (
	_ layout.PaneContent = (*placeholderPane)(nil)
	_ layout.PaneContent = (*shellPane)(nil)
)
