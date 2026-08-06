//go:build linux

package term

// Linux ioctl request numbers for termios and window size.
const (
	ioctlGetTermios = 0x5401 // TCGETS
	ioctlSetTermios = 0x5402 // TCSETS
	ioctlGetWinsize = 0x5413 // TIOCGWINSZ
)
