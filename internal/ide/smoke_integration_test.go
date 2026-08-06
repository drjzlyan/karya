//go:build integration && (darwin || linux)

package ide_test

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/drjzlyan/karya/internal/assets"
	"github.com/drjzlyan/karya/internal/pty"
)

// TestEngineConfigValid extracts the embedded engine config and confirms Neovim
// loads it with no Lua error.
func TestEngineConfigValid(t *testing.T) {
	if _, err := exec.LookPath("nvim"); err != nil {
		t.Skip("nvim not on PATH")
	}
	dir := t.TempDir()
	if err := assets.ExtractNvimEngine(dir); err != nil {
		t.Fatalf("extract engine config: %v", err)
	}
	init := filepath.Join(dir, "init.lua")
	out, _ := exec.Command("nvim", "--headless", "-u", init, "-c", "qa!").CombinedOutput()
	if bytes.Contains(out, []byte("E5")) || bytes.Contains(out, []byte("Error executing")) {
		t.Fatalf("engine config produced an error:\n%s", out)
	}
}

// quitAndWait sends Ctrl+Space Q, keeps draining the pty (so karya's teardown
// output never blocks on a full buffer), and waits for the process to exit.
func quitAndWait(t *testing.T, p *pty.PTY, cmd *exec.Cmd) {
	t.Helper()
	go func() { _, _ = io.Copy(io.Discard, p) }()
	_, _ = p.Write([]byte{0x00, 'Q'}) // Ctrl+Space then Q
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("karya tui did not quit after Ctrl+Space Q")
	}
}

// buildKarya compiles the karya binary into a temp dir and returns its path.
func buildKarya(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "karya")
	build := exec.Command("go", "build", "-o", bin, "github.com/drjzlyan/karya")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("build karya: %v", err)
	}
	return bin
}

// TestTUISmoke builds karya, launches `karya tui` on a real pseudo-terminal,
// waits for it to render its pane frame, then quits it with Ctrl+Space Q.
func TestTUISmoke(t *testing.T) {
	bin := buildKarya(t)

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

	quitAndWait(t, p, cmd)
}

// TestTUIEditorSmoke opens a file in the embedded Neovim editor pane and checks
// its content renders through the msgpack-RPC + Grid pipeline.
func TestTUIEditorSmoke(t *testing.T) {
	if _, err := exec.LookPath("nvim"); err != nil {
		t.Skip("nvim not on PATH")
	}
	bin := buildKarya(t)

	dir := t.TempDir()
	file := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(file, []byte("KARYA_EDITOR_SMOKE hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "tui", file)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "SHELL=/bin/sh")
	p, err := pty.Start(cmd, 80, 24)
	if err != nil {
		t.Skipf("pty unavailable: %v", err)
	}
	defer func() { _ = p.Close() }()

	got := readUntilByte(t, p, []byte("KARYA_EDITOR_SMOKE"), 8*time.Second)
	if !bytes.Contains(got, []byte("KARYA_EDITOR_SMOKE")) {
		t.Fatalf("editor did not render file content; output:\n%q", got)
	}

	quitAndWait(t, p, cmd)
}

// TestTUIEditorInput verifies real keystrokes reach the embedded editor and the
// screen updates: it types text in insert mode and checks it renders.
func TestTUIEditorInput(t *testing.T) {
	if _, err := exec.LookPath("nvim"); err != nil {
		t.Skip("nvim not on PATH")
	}
	bin := buildKarya(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(file, []byte("start\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "tui", file)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "SHELL=/bin/sh")
	p, err := pty.Start(cmd, 80, 24)
	if err != nil {
		t.Skipf("pty unavailable: %v", err)
	}
	defer func() { _ = p.Close() }()

	// keep draining so nothing blocks
	var out syncBytes
	go func() { _, _ = io.Copy(&out, p) }()

	// wait for initial render
	waitBytes(t, &out, []byte("start"), 8*time.Second)
	out.reset()

	// Type: o (open line) TYPEDLINE <Esc> — regular keys, not a leader.
	_, _ = p.Write([]byte("oTYPEDLINE\x1b"))
	if !waitBytes(t, &out, []byte("TYPEDLINE"), 5*time.Second) {
		t.Fatalf("typed text did not render; recent output:\n%q", out.tail(400))
	}
}

type syncBytes struct {
	mu  sync.Mutex
	buf []byte
}

func (s *syncBytes) Write(p []byte) (int, error) {
	s.mu.Lock()
	s.buf = append(s.buf, p...)
	s.mu.Unlock()
	return len(p), nil
}
func (s *syncBytes) contains(b []byte) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return bytes.Contains(s.buf, b)
}
func (s *syncBytes) reset() { s.mu.Lock(); s.buf = nil; s.mu.Unlock() }
func (s *syncBytes) tail(n int) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.buf) > n {
		return append([]byte(nil), s.buf[len(s.buf)-n:]...)
	}
	return append([]byte(nil), s.buf...)
}

func waitBytes(t *testing.T, s *syncBytes, want []byte, d time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if s.contains(want) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return s.contains(want)
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
