package nvimrpc

import (
	"io"
	"sync"
	"testing"
	"time"

	"github.com/drjzlyan/karya/internal/nvimrpc/msgpack"
)

// fakePeer is a scripted RPC peer standing in for nvim over pipes.
type fakePeer struct {
	toClient *io.PipeWriter
	notes    chan []any // client -> peer notifications
	mu       sync.Mutex
}

// startFake wires a Client to a fake peer. reply produces (result, errVal) for
// each request the client sends.
func startFake(t *testing.T, reply func(method string, params []any) (any, any)) (*Client, *fakePeer, func()) {
	t.Helper()
	cr, fw := io.Pipe() // client reads what the peer writes
	fr, cw := io.Pipe() // peer reads what the client writes

	c := NewClient(cr, cw)
	c.Serve()
	fp := &fakePeer{toClient: fw, notes: make(chan []any, 16)}

	go func() {
		dec := msgpack.NewDecoder(fr)
		for {
			v, err := dec.Decode()
			if err != nil {
				return
			}
			msg, _ := v.([]any)
			if len(msg) == 0 {
				continue
			}
			switch toInt(msg[0]) {
			case typeRequest:
				id := msg[1]
				method, _ := msg[2].(string)
				params, _ := msg[3].([]any)
				res, errVal := reply(method, params)
				b, _ := msgpack.Marshal([]any{typeResponse, id, errVal, res})
				fp.mu.Lock()
				_, _ = fp.toClient.Write(b)
				fp.mu.Unlock()
			case typeNotification:
				fp.notes <- msg
			}
		}
	}()

	cleanup := func() { _ = c.Close(); _ = fw.Close(); _ = cw.Close() }
	return c, fp, cleanup
}

// sendNotification pushes an inbound notification from the peer to the client.
func (fp *fakePeer) sendNotification(method string, params []any) {
	b, _ := msgpack.Marshal([]any{typeNotification, method, params})
	fp.mu.Lock()
	_, _ = fp.toClient.Write(b)
	fp.mu.Unlock()
}

func TestCallReturnsResult(t *testing.T) {
	c, _, cleanup := startFake(t, func(method string, params []any) (any, any) {
		return "pong", nil
	})
	defer cleanup()

	got, err := c.Call("ping")
	if err != nil {
		t.Fatal(err)
	}
	if got != "pong" {
		t.Fatalf("Call = %v want pong", got)
	}
}

func TestCallPassesArgs(t *testing.T) {
	var gotArgs []any
	c, _, cleanup := startFake(t, func(method string, params []any) (any, any) {
		gotArgs = params
		return int64(len(params)), nil
	})
	defer cleanup()

	_, err := c.Call("nvim_ui_attach", 80, 24, map[string]any{"ext_linegrid": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(gotArgs) != 3 || toInt(gotArgs[0]) != 80 || toInt(gotArgs[1]) != 24 {
		t.Fatalf("args not passed through: %v", gotArgs)
	}
	if _, ok := gotArgs[2].(map[string]any); !ok {
		t.Fatalf("opts map not passed: %T", gotArgs[2])
	}
}

func TestCallPropagatesError(t *testing.T) {
	c, _, cleanup := startFake(t, func(method string, params []any) (any, any) {
		return nil, "boom"
	})
	defer cleanup()

	if _, err := c.Call("explode"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestNotifyReachesPeer(t *testing.T) {
	c, fp, cleanup := startFake(t, func(method string, params []any) (any, any) { return nil, nil })
	defer cleanup()

	if err := c.Input("iabc"); err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-fp.notes:
		if msg[1] != "nvim_input" {
			t.Fatalf("method = %v", msg[1])
		}
		params := msg[2].([]any)
		if params[0] != "iabc" {
			t.Fatalf("input arg = %v", params[0])
		}
	case <-time.After(time.Second):
		t.Fatal("peer never received the notification")
	}
}

func TestInboundNotificationDispatched(t *testing.T) {
	c, fp, cleanup := startFake(t, func(method string, params []any) (any, any) { return nil, nil })
	defer cleanup()

	got := make(chan []any, 1)
	c.SetNotificationHandler(func(method string, params []any) {
		if method == "redraw" {
			got <- params
		}
	})
	fp.sendNotification("redraw", []any{[]any{"flush"}})

	select {
	case params := <-got:
		if len(params) != 1 {
			t.Fatalf("params = %v", params)
		}
	case <-time.After(time.Second):
		t.Fatal("notification not dispatched")
	}
}

func TestConcurrentCalls(t *testing.T) {
	// Peer echoes the method name, so each caller can verify it got its own reply.
	c, _, cleanup := startFake(t, func(method string, params []any) (any, any) {
		return method, nil
	})
	defer cleanup()

	var wg sync.WaitGroup
	methods := []string{"a", "b", "c", "d", "e"}
	for _, m := range methods {
		wg.Add(1)
		go func(m string) {
			defer wg.Done()
			got, err := c.Call(m)
			if err != nil || got != m {
				t.Errorf("Call(%s) = %v, %v", m, got, err)
			}
		}(m)
	}
	wg.Wait()
}

func TestCallFailsAfterClose(t *testing.T) {
	c, _, cleanup := startFake(t, func(method string, params []any) (any, any) { return nil, nil })
	cleanup() // closes the connection
	if _, err := c.Call("ping"); err == nil {
		t.Fatal("expected error after close")
	}
}
