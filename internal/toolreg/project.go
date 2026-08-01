package toolreg

import (
	"os"
	"path/filepath"
)

// projectConfigNames are the per-project version files karya honors. When one is
// present, its runtime versions layer over karya's global managed versions —
// mise resolves them natively when run with the project as the working dir.
var projectConfigNames = []string{"mise.toml", ".mise.toml", ".tool-versions"}

// ProjectEnv describes a detected project whose own runtime versions should take
// precedence over karya's global managed versions. It is data only; the layering
// is applied by config.Paths.EnvForProject (env) and the resolver (Go-side).
type ProjectEnv struct {
	// Root is the project directory that owns the version config.
	Root string
	// Configs are the absolute paths of the version files found at Root.
	Configs []string
}

// DetectProject walks up from dir looking for the nearest directory that pins its
// own runtime versions (mise.toml/.mise.toml/.tool-versions) and returns it. It
// stops at the filesystem root. It reports false when no project version config
// is found, in which case karya's global managed versions apply.
func DetectProject(dir string) (*ProjectEnv, bool) {
	dir = filepath.Clean(dir)
	for {
		if configs := projectConfigsIn(dir); len(configs) > 0 {
			return &ProjectEnv{Root: dir, Configs: configs}, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, false // reached the filesystem root
		}
		dir = parent
	}
}

// projectConfigsIn returns the version-config files present directly in dir.
func projectConfigsIn(dir string) []string {
	var out []string
	for _, name := range projectConfigNames {
		p := filepath.Join(dir, name)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			out = append(out, p)
		}
	}
	return out
}
