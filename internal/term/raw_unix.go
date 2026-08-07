//go:build darwin || linux

package term

import (
	"syscall"
	"unsafe"
)

// State holds the terminal settings captured before entering raw mode so they
// can be restored on exit.
type State struct{ termios syscall.Termios }

// MakeRaw puts the terminal referred to by fd into raw mode (no echo, no line
// buffering, no signal generation, byte-at-a-time reads) and returns the prior
// state for Restore. The platform-specific ioctl request numbers live in the
// per-OS files; everything else is shared.
func MakeRaw(fd int) (*State, error) {
	old, err := getTermios(fd)
	if err != nil {
		return nil, err
	}
	raw := *old
	raw.Iflag &^= syscall.BRKINT | syscall.ICRNL | syscall.INPCK | syscall.ISTRIP | syscall.IXON
	raw.Oflag &^= syscall.OPOST
	raw.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	raw.Cflag &^= syscall.CSIZE | syscall.PARENB
	raw.Cflag |= syscall.CS8
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	if err := setTermios(fd, &raw); err != nil {
		return nil, err
	}
	return &State{termios: *old}, nil
}

// Restore returns the terminal to the state captured by MakeRaw.
func Restore(fd int, st *State) error {
	if st == nil {
		return nil
	}
	return setTermios(fd, &st.termios)
}

// Size returns the terminal's width and height in cells.
func Size(fd int) (cols, rows int, err error) {
	var ws winsize
	if err := ioctl(fd, ioctlGetWinsize, unsafe.Pointer(&ws)); err != nil {
		return 0, 0, err
	}
	return int(ws.Col), int(ws.Row), nil
}

// IsTerminal reports whether fd refers to a terminal.
func IsTerminal(fd int) bool {
	_, err := getTermios(fd)
	return err == nil
}

type winsize struct {
	Row, Col, Xpixel, Ypixel uint16
}

func getTermios(fd int) (*syscall.Termios, error) {
	var t syscall.Termios
	if err := ioctl(fd, ioctlGetTermios, unsafe.Pointer(&t)); err != nil {
		return nil, err
	}
	return &t, nil
}

func setTermios(fd int, t *syscall.Termios) error {
	return ioctl(fd, ioctlSetTermios, unsafe.Pointer(t))
}

func ioctl(fd int, req uint, arg unsafe.Pointer) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}
