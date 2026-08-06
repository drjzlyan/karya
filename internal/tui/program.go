package tui

import (
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/drjzlyan/karya/internal/cellbuf"
	"github.com/drjzlyan/karya/internal/term"
)

// escTimeout is how long the loop waits for the rest of an escape sequence
// before treating a pending ESC as the Esc key.
const escTimeout = 50 * time.Millisecond

// Program runs a Model against a terminal: it sets up raw mode and the alt
// screen, decodes input into messages, dispatches them to the Model, runs the
// resulting commands off the render path, and repaints via cellbuf diffs.
type Program struct {
	model Model

	in     io.Reader
	out    io.Writer
	fd     int
	useRaw bool

	forceSize bool
	cols      int
	rows      int

	caps   term.Caps
	output *term.Output
	prev   *cellbuf.Buffer

	msgCh chan Msg
	rawCh chan []byte
	done  chan struct{}

	closeOnce sync.Once
}

// Option configures a Program.
type Option func(*Program)

// WithInput sets the input source (default os.Stdin).
func WithInput(r io.Reader) Option { return func(p *Program) { p.in = r } }

// WithOutput sets the output sink (default os.Stdout).
func WithOutput(w io.Writer) Option { return func(p *Program) { p.out = w } }

// WithoutRawMode disables raw-mode/alt-screen setup. Used in tests that drive the
// loop over pipes rather than a real terminal.
func WithoutRawMode() Option { return func(p *Program) { p.useRaw = false } }

// WithSize forces a fixed screen size instead of querying the terminal. Used in
// tests and headless rendering.
func WithSize(cols, rows int) Option {
	return func(p *Program) { p.forceSize, p.cols, p.rows = true, cols, rows }
}

// WithCaps sets the terminal capabilities used for output encoding.
func WithCaps(caps term.Caps) Option {
	return func(p *Program) { p.caps = caps }
}

// NewProgram builds a Program for model. By default it reads os.Stdin, writes
// os.Stdout, and uses raw mode.
func NewProgram(model Model, opts ...Option) *Program {
	p := &Program{
		model:  model,
		in:     os.Stdin,
		out:    os.Stdout,
		fd:     int(os.Stdin.Fd()),
		useRaw: true,
		cols:   80,
		rows:   24,
		caps:   term.Caps{Palette: 256},
		msgCh:  make(chan Msg, 64),
		rawCh:  make(chan []byte, 16),
		done:   make(chan struct{}),
	}
	for _, o := range opts {
		o(p)
	}
	p.output = term.NewOutput(p.out, p.caps)
	return p
}

// Send delivers a message into the loop from any goroutine. It is a no-op once
// the program has stopped.
func (p *Program) Send(m Msg) {
	select {
	case p.msgCh <- m:
	case <-p.done:
	}
}

// Run starts the loop and blocks until the Model quits or input ends. It returns
// the final model.
func (p *Program) Run() (Model, error) {
	if p.useRaw {
		state, err := term.MakeRaw(p.fd)
		if err != nil {
			return p.model, err
		}
		defer func() { _ = term.Restore(p.fd, state) }()
		p.output.EnterAltScreen()
		p.output.HideCursor()
		p.output.EnableBracketedPaste()
		p.output.EnableMouse()
		_ = p.output.Flush()
		defer p.teardown()
	}

	if !p.forceSize && p.useRaw {
		if c, r, err := term.Size(p.fd); err == nil && c > 0 && r > 0 {
			p.cols, p.rows = c, r
		}
	}

	go p.readInput()
	winch := p.watchResize()
	defer signal.Stop(winch)

	if cmd := p.model.Init(); cmd != nil {
		p.runCmd(cmd)
	}
	p.render()

	dec := term.NewDecoder()
	var escTimer <-chan time.Time

	for {
		select {
		case <-p.done:
			return p.model, nil
		case data := <-p.rawCh:
			for _, ev := range dec.Feed(data) {
				if p.dispatch(ev) {
					return p.model, nil
				}
			}
			if dec.Buffered() > 0 {
				escTimer = time.After(escTimeout)
			} else {
				escTimer = nil
			}
			p.render()
		case <-escTimer:
			escTimer = nil
			for _, ev := range dec.Flush() {
				if p.dispatch(ev) {
					return p.model, nil
				}
			}
			p.render()
		case m := <-p.msgCh:
			if p.dispatch(m) {
				return p.model, nil
			}
			p.render()
		case <-winch:
			if c, r, err := term.Size(p.fd); err == nil && c > 0 && r > 0 {
				p.cols, p.rows = c, r
				p.prev = nil // force a full repaint at the new size
				if p.dispatch(term.ResizeEvent{Cols: c, Rows: r}) {
					return p.model, nil
				}
				p.render()
			}
		}
	}
}

// dispatch applies one message to the model, runs any command, and reports
// whether the program should stop.
func (p *Program) dispatch(m Msg) (quit bool) {
	if _, ok := m.(quitMsg); ok {
		p.close()
		return true
	}
	if b, ok := m.(batchMsg); ok {
		for _, c := range b.cmds {
			p.runCmd(c)
		}
		return false
	}
	next, cmd := p.model.Update(m)
	p.model = next
	if cmd != nil {
		p.runCmd(cmd)
	}
	return false
}

// runCmd executes a command off the render path and feeds its result back in.
func (p *Program) runCmd(cmd Cmd) {
	go func() {
		if msg := cmd(); msg != nil {
			p.Send(msg)
		}
	}()
}

// render draws the current model, diffs against the previous frame, and flushes.
func (p *Program) render() {
	buf := cellbuf.New(p.cols, p.rows)
	p.model.View(buf)
	spans := cellbuf.Diff(p.prev, buf)
	if len(spans) > 0 {
		p.output.Render(spans)
		_ = p.output.Flush()
	}
	p.prev = buf
}

// readInput copies input into rawCh until the source ends or the program stops.
func (p *Program) readInput() {
	buf := make([]byte, 4096)
	for {
		n, err := p.in.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			select {
			case p.rawCh <- chunk:
			case <-p.done:
				return
			}
		}
		if err != nil {
			return
		}
	}
}

// watchResize returns a channel that fires on SIGWINCH (only in raw mode).
func (p *Program) watchResize() chan os.Signal {
	ch := make(chan os.Signal, 1)
	if p.useRaw {
		signal.Notify(ch, syscall.SIGWINCH)
	}
	return ch
}

// teardown restores terminal modes on exit.
func (p *Program) teardown() {
	p.output.DisableMouse()
	p.output.DisableBracketedPaste()
	p.output.ShowCursor()
	p.output.ExitAltScreen()
	_ = p.output.Flush()
}

func (p *Program) close() {
	p.closeOnce.Do(func() { close(p.done) })
}

// Size reports the current screen size.
func (p *Program) Size() (cols, rows int) { return p.cols, p.rows }
