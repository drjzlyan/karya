//go:build darwin || linux

// Package pty hosts child processes (shells and interactive agent CLIs) on
// pseudo-terminals — the job tmux used to do, now owned by karya (DESIGN.md
// §6.1). A PTY exposes the master side as an io.ReadWriteCloser; a pane feeds
// the child's output through internal/pty/vt to render it, and writes forwarded
// keys back. The low-level pty allocation is build-tagged per OS; the rest is
// shared here.
package pty

import (
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

// PTY is a running child attached to a pseudo-terminal. Read/Write operate on the
// master side.
type PTY struct {
	master *os.File
	cmd    *exec.Cmd
}

// Start launches cmd attached to a new pseudo-terminal sized cols×rows and
// returns the PTY. The child becomes a session leader with the pty as its
// controlling terminal.
func Start(cmd *exec.Cmd, cols, rows int) (*PTY, error) {
	master, slave, err := open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = slave.Close() }()

	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
	cmd.SysProcAttr.Setctty = true

	if err := cmd.Start(); err != nil {
		_ = master.Close()
		return nil, err
	}
	p := &PTY{master: master, cmd: cmd}
	if cols > 0 && rows > 0 {
		_ = p.Resize(cols, rows)
	}
	return p, nil
}

// Read reads child output from the master side.
func (p *PTY) Read(b []byte) (int, error) { return p.master.Read(b) }

// Write sends input to the child via the master side.
func (p *PTY) Write(b []byte) (int, error) { return p.master.Write(b) }

// Resize sets the pseudo-terminal window size, which signals SIGWINCH to the
// child.
func (p *PTY) Resize(cols, rows int) error {
	ws := winsize{Row: uint16(rows), Col: uint16(cols)}
	return ioctl(int(p.master.Fd()), tiocswinsz, unsafe.Pointer(&ws))
}

// Wait waits for the child to exit.
func (p *PTY) Wait() error { return p.cmd.Wait() }

// Close closes the master side (which ends the child's I/O).
func (p *PTY) Close() error { return p.master.Close() }

type winsize struct {
	Row, Col, Xpixel, Ypixel uint16
}

func ioctl(fd int, req uint, arg unsafe.Pointer) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}
