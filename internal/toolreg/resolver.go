package toolreg

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/drjzlyan/karya/internal/config"
)

// Source records where a resolved tool came from, in priority order. It lets the
// IDE (and diagnostics) explain which copy of a tool is in effect and prefer a
// project-pinned version over the global managed one over whatever is on PATH.
type Source int

const (
	// SourceProject is a project-pinned runtime (mise.toml/.tool-versions).
	// Reserved for the per-project isolation phase; the resolver does not yet
	// emit it.
	SourceProject Source = iota
	// SourceManaged is karya's isolated managed prefix (tool bin or mise shims).
	SourceManaged
	// SourceSystem is a fallback found on the ambient PATH.
	SourceSystem
	// SourceMissing means the tool could not be resolved anywhere.
	SourceMissing
)

// String renders a Source for manifests and diagnostics.
func (s Source) String() string {
	switch s {
	case SourceProject:
		return "project"
	case SourceManaged:
		return "managed"
	case SourceSystem:
		return "system"
	default:
		return "missing"
	}
}

// Resolved is the outcome of resolving a tool: its absolute path, the version if
// known (populated by the version manager phase), and where it came from.
type Resolved struct {
	ID      string
	Path    string
	Version string
	Source  Source
}

// Resolver maps a tool ID (or a bare executable name) to the executable karya
// should run, preferring karya's isolated managed prefix over the ambient PATH
// and never consulting Homebrew or the user's global environment. It is the
// single seam every execution path in the IDE goes through, so tool location
// logic lives in one place instead of being re-derived at each call site.
type Resolver struct {
	paths config.Paths
	reg   *Registry
	// managedDirs are searched, in order, for a managed executable.
	managedDirs []string
	// lookPath resolves a bare name on PATH; injectable for tests.
	lookPath func(string) (string, error)
}

// NewResolver builds a resolver over karya's managed prefix and the registry.
func NewResolver(p config.Paths, r *Registry) *Resolver {
	return &Resolver{
		paths:       p,
		reg:         r,
		managedDirs: []string{p.ToolsBin(), p.MiseShims()},
		lookPath:    exec.LookPath,
	}
}

// Resolve returns the executable for a tool. id may be a registry ID (resolved
// to its Executable/Artifact) or a bare command name. Resolution order is
// managed prefix → ambient PATH → missing. Artifact-only tools (e.g. lombok.jar)
// resolve to their artifact path under the tool prefix.
func (rv *Resolver) Resolve(id string) (Resolved, bool) {
	exe := id
	var artifact string
	if t, ok := rv.reg.Get(id); ok {
		if t.Executable != "" {
			exe = t.Executable
		} else {
			exe = ""
		}
		artifact = t.Artifact
	}

	if artifact != "" {
		if p := filepath.Join(rv.paths.ToolsDir(), artifact); fileExists(p) {
			return Resolved{ID: id, Path: p, Source: SourceManaged}, true
		}
	}
	if exe != "" {
		for _, dir := range rv.managedDirs {
			if p := filepath.Join(dir, exe); fileExists(p) {
				return Resolved{ID: id, Path: p, Source: SourceManaged}, true
			}
		}
		if p, err := rv.lookPath(exe); err == nil {
			return Resolved{ID: id, Path: p, Source: SourceSystem}, true
		}
	}
	return Resolved{ID: id, Source: SourceMissing}, false
}

// Path returns the resolved executable path, or "" when the tool is missing.
func (rv *Resolver) Path(id string) string {
	if r, ok := rv.Resolve(id); ok {
		return r.Path
	}
	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
