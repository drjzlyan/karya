//go:build integration

package nvimrpc

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// guardedGrid serializes Grid access between the RPC reader goroutine and the
// test goroutine.
type guardedGrid struct {
	mu sync.Mutex
	g  *Grid
}

func (gg *guardedGrid) apply(params []any) {
	gg.mu.Lock()
	gg.g.HandleRedraw(params)
	gg.mu.Unlock()
}

func (gg *guardedGrid) text() string {
	gg.mu.Lock()
	defer gg.mu.Unlock()
	return gg.g.Buffer().String()
}

func startNvim(t *testing.T) (*Client, *guardedGrid) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	c, err := Start(ctx, []string{"nvim", "--embed", "--clean", "-n"}, nil)
	if err != nil {
		t.Skipf("nvim unavailable: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	gg := &guardedGrid{g: NewGrid(80, 24)}
	c.SetNotificationHandler(func(method string, params []any) {
		if method == "redraw" {
			gg.apply(params)
		}
	})
	if err := c.UIAttach(80, 24, map[string]any{"ext_linegrid": true, "rgb": true}); err != nil {
		t.Fatalf("ui_attach: %v", err)
	}
	return c, gg
}

func waitFor(t *testing.T, cond func() bool, d time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return cond()
}

func TestNvimAPIInfo(t *testing.T) {
	c, _ := startNvim(t)
	info, err := c.Call("nvim_get_api_info")
	if err != nil {
		t.Fatalf("get_api_info: %v", err)
	}
	arr, ok := info.([]any)
	if !ok || len(arr) != 2 {
		t.Fatalf("api info shape: %T %v", info, info)
	}
}

func TestNvimTypingRendersToGrid(t *testing.T) {
	c, gg := startNvim(t)

	// Wait for the initial screen to render.
	if !waitFor(t, func() bool { return gg.text() != "" }, 2*time.Second) {
		t.Fatal("no initial redraw")
	}
	// Insert text, leave insert mode.
	if err := c.Input("ihello karya"); err != nil {
		t.Fatal(err)
	}
	if err := c.Input("<Esc>"); err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, func() bool { return strings.Contains(gg.text(), "hello karya") }, 3*time.Second) {
		t.Fatalf("typed text not rendered to grid; screen:\n%s", gg.text())
	}
}

func TestNvimResize(t *testing.T) {
	c, _ := startNvim(t)
	if err := c.TryResize(100, 30); err != nil {
		t.Fatalf("try_resize: %v", err)
	}
}

func TestNvimSetOption(t *testing.T) {
	c, _ := startNvim(t)
	if err := c.SetOption("number", true); err != nil {
		t.Fatalf("set_option: %v", err)
	}
}
