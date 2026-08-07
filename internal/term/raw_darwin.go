//go:build darwin

package term

// Darwin ioctl request numbers for termios and window size.
const (
	ioctlGetTermios = 0x40487413 // TIOCGETA
	ioctlSetTermios = 0x80487414 // TIOCSETA
	ioctlGetWinsize = 0x40087468 // TIOCGWINSZ
)
