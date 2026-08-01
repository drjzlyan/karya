package toolreg

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Manifest is the resolved-tool map karya writes for the embedded Neovim config.
// The editor reads it to launch managed tools (jdtls, lombok, formatters) by
// absolute path from karya's isolated prefix, instead of probing global
// locations — the mechanism that lets karya keep tool discovery isolated on both
// the Go and the Lua side.
type Manifest struct {
	// Tools maps a tool ID to its resolved location.
	Tools map[string]ManifestEntry `json:"tools"`
}

// ManifestEntry is one resolved tool: its absolute path and where it came from.
type ManifestEntry struct {
	Path   string `json:"path"`
	Source string `json:"source"`
}

// Manifest resolves every registry tool that is currently present and returns
// the map. Tools that do not resolve are omitted, so a stale entry never points
// at something that is gone; the editor falls back to PATH for anything absent.
func (rv *Resolver) Manifest() Manifest {
	m := Manifest{Tools: make(map[string]ManifestEntry)}
	for _, t := range rv.reg.All() {
		if r, ok := rv.Resolve(t.ID); ok {
			m.Tools[t.ID] = ManifestEntry{Path: r.Path, Source: r.Source.String()}
		}
	}
	return m
}

// WriteManifest writes the manifest as indented JSON to path, creating parent
// dirs. It is best-effort: callers treat a write error as non-fatal since the
// editor degrades to PATH resolution without it.
func WriteManifest(path string, m Manifest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
