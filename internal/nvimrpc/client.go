// Package nvimrpc embeds Neovim as karya's text-editing engine: it spawns
// `nvim --embed`, speaks msgpack-RPC over stdio, attaches a UI, reduces redraw
// events into a Grid (grid.go), and forwards input — so karya renders Neovim's
// screen inside its own cell buffer and owns all keybinding (DESIGN.md §6.3).
//
// The client is transport-agnostic (NewClient over any reader/writer) so it is
// testable against a scripted fake peer; Start spawns the real process.
package nvimrpc

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/drjzlyan/karya/internal/nvimrpc/msgpack"
)

// message type tags (msgpack-RPC).
const (
	typeRequest      = 0
	typeResponse     = 1
	typeNotification = 2
)

// NotifyFunc handles an inbound notification (e.g. "redraw").
type NotifyFunc func(method string, params []any)

type pending struct {
	ch chan result
}

type result struct {
	value any
	err   error
}

// Client is a msgpack-RPC peer for an embedded Neovim.
type Client struct {
	w   io.Writer
	dec *msgpack.Decoder
	cmd *exec.Cmd
	cl  io.Closer

	writeMu sync.Mutex

	mu      sync.Mutex
	nextID  uint32
	pending map[uint32]*pending
	onNote  NotifyFunc

	closeOnce sync.Once
	done      chan struct{}
}

// NewClient builds a client over an existing transport: it reads responses/
// notifications from r and writes requests to w. Call Serve to start the reader.
func NewClient(r io.Reader, w io.Writer) *Client {
	return &Client{
		w:       w,
		dec:     msgpack.NewDecoder(r),
		pending: map[uint32]*pending{},
		done:    make(chan struct{}),
	}
}

// Start spawns argv (typically {"nvim", "--embed"}) with extraEnv appended and
// returns a connected, serving client.
func Start(ctx context.Context, argv, extraEnv []string) (*Client, error) {
	if len(argv) == 0 {
		return nil, fmt.Errorf("nvimrpc: empty argv")
	}
	if _, err := exec.LookPath(argv[0]); err != nil {
		return nil, fmt.Errorf("nvimrpc: %q not on PATH: %w", argv[0], err)
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	if len(extraEnv) > 0 {
		cmd.Env = append(cmd.Environ(), extraEnv...)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	c := NewClient(stdout, stdin)
	c.cmd = cmd
	c.cl = stdin
	c.Serve()
	return c, nil
}

// SetNotificationHandler registers the handler for inbound notifications. It must
// be set before UIAttach so no redraw is missed.
func (c *Client) SetNotificationHandler(fn NotifyFunc) {
	c.mu.Lock()
	c.onNote = fn
	c.mu.Unlock()
}

// Serve starts the background reader. Start calls it automatically.
func (c *Client) Serve() { go c.readLoop() }

// Done is closed when the connection ends (nvim exits or the stream errors).
func (c *Client) Done() <-chan struct{} { return c.done }

// Call invokes method with args and waits for the result.
func (c *Client) Call(method string, args ...any) (any, error) {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	p := &pending{ch: make(chan result, 1)}
	c.pending[id] = p
	c.mu.Unlock()

	if err := c.write([]any{typeRequest, id, method, args}); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, err
	}
	select {
	case r := <-p.ch:
		return r.value, r.err
	case <-c.done:
		return nil, fmt.Errorf("nvimrpc: connection closed")
	}
}

// Notify sends a fire-and-forget notification (no reply awaited).
func (c *Client) Notify(method string, args ...any) error {
	return c.write([]any{typeNotification, method, args})
}

func (c *Client) write(msg []any) error {
	b, err := msgpack.Marshal(msg)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err = c.w.Write(b)
	return err
}

func (c *Client) readLoop() {
	for {
		v, err := c.dec.Decode()
		if err != nil {
			c.shutdown(err)
			return
		}
		msg, ok := v.([]any)
		if !ok || len(msg) == 0 {
			continue
		}
		switch toInt(msg[0]) {
		case typeResponse:
			c.handleResponse(msg)
		case typeNotification:
			c.handleNotification(msg)
		case typeRequest:
			c.handleInboundRequest(msg)
		}
	}
}

func (c *Client) handleResponse(msg []any) {
	if len(msg) < 4 {
		return
	}
	id := uint32(toInt(msg[1]))
	c.mu.Lock()
	p := c.pending[id]
	delete(c.pending, id)
	c.mu.Unlock()
	if p == nil {
		return
	}
	var r result
	if msg[2] != nil {
		r.err = fmt.Errorf("nvimrpc: %v", msg[2])
	} else {
		r.value = msg[3]
	}
	p.ch <- r
}

func (c *Client) handleNotification(msg []any) {
	if len(msg) < 3 {
		return
	}
	method, _ := msg[1].(string)
	params, _ := msg[2].([]any)
	c.mu.Lock()
	fn := c.onNote
	c.mu.Unlock()
	if fn != nil {
		fn(method, params)
	}
}

// handleInboundRequest replies to requests from nvim; karya registers no server
// methods, so it returns a not-implemented error to keep nvim unblocked.
func (c *Client) handleInboundRequest(msg []any) {
	if len(msg) < 2 {
		return
	}
	id := toInt(msg[1])
	_ = c.write([]any{typeResponse, id, "method not supported", nil})
}

func (c *Client) shutdown(err error) {
	c.closeOnce.Do(func() {
		close(c.done)
		c.mu.Lock()
		for id, p := range c.pending {
			p.ch <- result{err: fmt.Errorf("nvimrpc: closed: %w", err)}
			delete(c.pending, id)
		}
		c.mu.Unlock()
	})
}

// Close terminates the connection (and the process, if spawned).
func (c *Client) Close() error {
	if c.cl != nil {
		_ = c.cl.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_ = c.cmd.Wait()
	}
	c.shutdown(fmt.Errorf("closed by caller"))
	return nil
}

// --- Neovim UI convenience wrappers ---

// UIAttach attaches a UI of the given size with the given options (e.g.
// {"ext_linegrid": true, "rgb": true}).
func (c *Client) UIAttach(width, height int, opts map[string]any) error {
	_, err := c.Call("nvim_ui_attach", width, height, opts)
	return err
}

// TryResize resizes the attached UI.
func (c *Client) TryResize(width, height int) error {
	_, err := c.Call("nvim_ui_try_resize", width, height)
	return err
}

// Input forwards key input in Neovim notation (e.g. "iabc<Esc>"). It is sent as a
// notification for low latency.
func (c *Client) Input(keys string) error {
	return c.Notify("nvim_input", keys)
}

// Command runs an Ex command.
func (c *Client) Command(cmd string) error {
	_, err := c.Call("nvim_command", cmd)
	return err
}

// SetOption sets a global option value via nvim_set_option_value.
func (c *Client) SetOption(name string, value any) error {
	_, err := c.Call("nvim_set_option_value", name, value, map[string]any{})
	return err
}
