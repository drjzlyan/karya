//go:build darwin || linux

package pty

import (
	"bytes"
	"os/exec"
	"testing"
	"time"

	"github.com/drjzlyan/karya/internal/pty/vt"
)

func TestPTYCapturesChildOutput(t *testing.T) {
	cmd := exec.Command("sh", "-c", "printf hello")
	p, err := Start(cmd, 40, 10)
	if err != nil {
		t.Skipf("pty unavailable in this environment: %v", err)
	}
	defer func() { _ = p.Close() }()

	got := readUntil(t, p, []byte("hello"), 3*time.Second)
	if !bytes.Contains(got, []byte("hello")) {
		t.Fatalf("did not capture child output, got %q", got)
	}
}

func TestPTYResize(t *testing.T) {
	cmd := exec.Command("sh", "-c", "sleep 0.2")
	p, err := Start(cmd, 40, 10)
	if err != nil {
		t.Skipf("pty unavailable: %v", err)
	}
	defer func() { _ = p.Close() }()
	if err := p.Resize(100, 30); err != nil {
		t.Fatalf("resize failed: %v", err)
	}
	_ = p.Wait()
}

func TestPTYIntoVTScreen(t *testing.T) {
	// End-to-end: child output flows through the vt emulator into a screen.
	cmd := exec.Command("sh", "-c", "printf 'line1\\r\\nline2'")
	p, err := Start(cmd, 20, 5)
	if err != nil {
		t.Skipf("pty unavailable: %v", err)
	}
	defer func() { _ = p.Close() }()

	screen := vt.New(20, 5)
	raw := readUntil(t, p, []byte("line2"), 3*time.Second)
	_, _ = screen.Write(raw)
	out := screen.String()
	if out != "line1\nline2" {
		t.Fatalf("vt screen from pty = %q want %q", out, "line1\nline2")
	}
}

// readUntil reads from p until want appears or the timeout elapses.
func readUntil(t *testing.T, p *PTY, want []byte, timeout time.Duration) []byte {
	t.Helper()
	done := make(chan []byte, 1)
	go func() {
		var acc []byte
		b := make([]byte, 256)
		for {
			n, err := p.Read(b)
			if n > 0 {
				acc = append(acc, b[:n]...)
				if bytes.Contains(acc, want) {
					break
				}
			}
			if err != nil {
				break
			}
		}
		done <- acc
	}()
	select {
	case acc := <-done:
		return acc
	case <-time.After(timeout):
		t.Fatal("timed out reading from pty")
		return nil
	}
}
