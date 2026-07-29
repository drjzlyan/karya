// Package version holds build metadata. The values are overridden at build time
// via -ldflags "-X github.com/drjzlyan/karya/internal/version.Version=..." so a
// released binary can report its exact version for `karya update`.
package version

import (
	"fmt"
	"runtime"
)

var (
	// Version is the semantic version of the build (e.g. "v0.1.0").
	Version = "dev"
	// Commit is the git commit the binary was built from.
	Commit = "none"
	// Date is the build timestamp (RFC3339).
	Date = "unknown"
)

// String returns a human-readable one-line version summary.
func String() string {
	return fmt.Sprintf("karya %s (commit %s, built %s, %s/%s, %s)",
		Version, Commit, Date, runtime.GOOS, runtime.GOARCH, runtime.Version())
}
