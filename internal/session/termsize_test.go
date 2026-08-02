package session

import "testing"

// TestTerminalSizeSane checks the ioctl helper never panics and, when it claims a
// terminal, returns positive dimensions. Under `go test` stdout is usually not a
// terminal, so this mainly exercises the graceful non-tty fallback (ok=false).
func TestTerminalSizeSane(t *testing.T) {
	cols, rows, ok := terminalSize()
	if ok && (cols <= 0 || rows <= 0) {
		t.Errorf("terminalSize reported ok with non-positive size %dx%d", cols, rows)
	}
}
