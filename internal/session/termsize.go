package session

import (
	"os"
	"syscall"
	"unsafe"
)

// terminalSize returns the size (in cells) of the controlling terminal via a
// TIOCGWINSZ ioctl on stdout. ok is false when stdout is not a terminal — under
// tests, a pipe, or redirected output — in which case the caller lets tmux pick
// its default size. karya targets only macOS and Linux, both of which define
// TIOCGWINSZ, so no per-OS build tags are needed.
func terminalSize() (cols, rows int, ok bool) {
	var ws struct {
		Row, Col, Xpixel, Ypixel uint16
	}
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		os.Stdout.Fd(),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(&ws)),
	)
	if errno != 0 || ws.Col == 0 || ws.Row == 0 {
		return 0, 0, false
	}
	return int(ws.Col), int(ws.Row), true
}
