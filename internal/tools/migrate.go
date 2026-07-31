package tools

import (
	"os"
	"path/filepath"

	"github.com/drjzlyan/karya/internal/config"
)

// layoutVersion is the current on-disk tool-layout revision. Bumping it triggers
// Migrate to run its steps again for existing installs.
const layoutVersion = "1"

// Migrate brings an existing karya prefix up to the current tool layout. It is
// idempotent and best-effort: it establishes the category/auxiliary directories
// and records the layout version, but never moves already-installed binaries —
// the resolver keeps the legacy shared bin as a permanent fallback, so existing
// tools keep working with zero action. A failure here is non-fatal; installs and
// resolution degrade to the legacy prefix.
func Migrate(p config.Paths) error {
	marker := filepath.Join(p.State, "layout.version")
	if data, err := os.ReadFile(marker); err == nil && string(trimSpace(data)) == layoutVersion {
		return nil
	}
	// Create the auxiliary dirs the new layout introduces. Category tool dirs are
	// created on demand by the installers, so they are not pre-created here.
	for _, d := range []string{p.DownloadsDir(), p.ToolsLogsDir()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		return err
	}
	return os.WriteFile(marker, []byte(layoutVersion+"\n"), 0o644)
}

// trimSpace trims trailing whitespace/newline from the marker contents without
// pulling in strings for one call site.
func trimSpace(b []byte) []byte {
	for len(b) > 0 {
		switch b[len(b)-1] {
		case '\n', '\r', ' ', '\t':
			b = b[:len(b)-1]
		default:
			return b
		}
	}
	return b
}
