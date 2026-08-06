//go:build linux

package pty

import (
	"os"
	"strconv"
	"syscall"
	"unsafe"
)

// Linux ioctl request numbers.
const (
	tiocsptlck = 0x40045431 // unlock pts
	tiocgptn   = 0x80045430 // get pts number
	tiocswinsz = 0x5414     // set window size
)

// open allocates a master/slave pty pair via /dev/ptmx.
func open() (master, slave *os.File, err error) {
	m, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	var unlock int32 // 0 => unlocked
	if err := ioctl(int(m.Fd()), tiocsptlck, unsafe.Pointer(&unlock)); err != nil {
		_ = m.Close()
		return nil, nil, err
	}
	var n uint32
	if err := ioctl(int(m.Fd()), tiocgptn, unsafe.Pointer(&n)); err != nil {
		_ = m.Close()
		return nil, nil, err
	}
	s, err := os.OpenFile("/dev/pts/"+strconv.Itoa(int(n)), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		_ = m.Close()
		return nil, nil, err
	}
	return m, s, nil
}
