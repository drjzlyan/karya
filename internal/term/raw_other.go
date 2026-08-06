//go:build !darwin && !linux

package term

import "errors"

// errUnsupported is returned by raw-mode operations on platforms karya does not
// target (only darwin and linux are supported). The stubs keep the package
// buildable for cross-compilation checks.
var errUnsupported = errors.New("term: raw mode unsupported on this platform")

// State is an empty placeholder on unsupported platforms.
type State struct{}

// MakeRaw is unsupported on non-unix platforms.
func MakeRaw(fd int) (*State, error) { return nil, errUnsupported }

// Restore is a no-op on unsupported platforms.
func Restore(fd int, st *State) error { return nil }

// Size is unsupported on non-unix platforms.
func Size(fd int) (cols, rows int, err error) { return 0, 0, errUnsupported }

// IsTerminal always reports false on unsupported platforms.
func IsTerminal(fd int) bool { return false }
