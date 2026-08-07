//go:build !darwin && !linux

package pty

import (
	"errors"
	"os/exec"
)

// errUnsupported is returned on platforms karya does not target (only darwin and
// linux). The stub keeps the package buildable for cross-compilation.
var errUnsupported = errors.New("pty: unsupported on this platform")

// PTY is an empty placeholder on unsupported platforms.
type PTY struct{}

// Start is unsupported on non-unix platforms.
func Start(cmd *exec.Cmd, cols, rows int) (*PTY, error) { return nil, errUnsupported }

// Read is unsupported.
func (p *PTY) Read(b []byte) (int, error) { return 0, errUnsupported }

// Write is unsupported.
func (p *PTY) Write(b []byte) (int, error) { return 0, errUnsupported }

// Resize is unsupported.
func (p *PTY) Resize(cols, rows int) error { return errUnsupported }

// Wait is unsupported.
func (p *PTY) Wait() error { return errUnsupported }

// Close is a no-op.
func (p *PTY) Close() error { return nil }
