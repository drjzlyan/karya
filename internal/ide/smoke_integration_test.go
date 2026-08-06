//go:build integration && (darwin || linux)

package ide_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/drjzlyan/karya/internal/pty"
)

// TestTUISmoke builds karya, launches `karya tui` on a real pseudo-terminal,
// waits for it to render its pane frame, then quits it with Ctrl+Space Q.
func TestTUISmoke(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "karya")
	build := exec.Command("go", "build", "-o", bin, "github.com/drjzlyan/karya")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("build karya: %v", err)
	}

	cmd := exec.Command(bin, "tui")
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "SHELL=/bin/sh")
	p, err := pty.Start(cmd, 80, 24)
	if err != nil {
		t.Skipf("pty unavailable: %v", err)
	}
	defer func() { _ = p.Close() }()

	// Read until the status line ("tab 1/1") appears, by which point the whole
	// first frame — including the pane frame — has been emitted.
	got := readUntilByte(t, p, []byte("tab 1/1"), 5*time.Second)
	if !bytes.Contains(got, []byte("┌")) {
		t.Fatalf("TUI did not render a pane frame; output:\n%q", got)
	}
	if !bytes.Contains(got, []byte("karya")) {
		t.Fatalf("status line not rendered; output:\n%q", got)
	}

	// Quit: Ctrl+Space (0x00) then Q.
	_, _ = p.Write([]byte{0x00, 'Q'})

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
		// exited cleanly
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("karya tui did not quit after Ctrl+Space Q")
	}
}

func readUntilByte(t *testing.T, p *pty.PTY, want []byte, timeout time.Duration) []byte {
	t.Helper()
	done := make(chan []byte, 1)
	go func() {
		var acc []byte
		b := make([]byte, 4096)
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
		return nil
	}
}
