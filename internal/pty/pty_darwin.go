//go:build darwin

package pty

import (
	"bytes"
	"os"
	"syscall"
	"unsafe"
)

// Darwin ioctl request numbers.
const (
	tiocptygrant = 0x20007454 // grant slave
	tiocptyunlk  = 0x20007452 // unlock slave
	tiocptygname = 0x40807453 // get slave name (128-byte buffer)
	tiocswinsz   = 0x80087467 // set window size
)

// open allocates a master/slave pty pair via /dev/ptmx.
func open() (master, slave *os.File, err error) {
	m, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	fd := int(m.Fd())
	if err := ioctl(fd, tiocptygrant, nil); err != nil {
		_ = m.Close()
		return nil, nil, err
	}
	if err := ioctl(fd, tiocptyunlk, nil); err != nil {
		_ = m.Close()
		return nil, nil, err
	}
	var buf [128]byte
	if err := ioctl(fd, tiocptygname, unsafe.Pointer(&buf[0])); err != nil {
		_ = m.Close()
		return nil, nil, err
	}
	name := string(buf[:bytes.IndexByte(buf[:], 0)])
	s, err := os.OpenFile(name, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		_ = m.Close()
		return nil, nil, err
	}
	return m, s, nil
}
