package toolreg

import (
	"os/exec"
	"strings"
)

// HealthStatus is a tool's health snapshot: whether it resolves, where, from
// which source, its version when known, and how to repair it when not. doctor and
// `karya tool` render these instead of failing silently.
type HealthStatus struct {
	ID         string
	Name       string
	Category   Category
	Installed  bool
	Version    string
	Location   string
	Source     Source
	RepairHint string
}

// HealthChecker validates tools against the resolver. The version probe is
// injectable so the checker can be exercised hermetically.
type HealthChecker struct {
	rv *Resolver
	// version returns a tool's version by running its executable; injectable for
	// tests. Default runs `<path> <args>` and returns the first output line.
	version func(path string, args []string) string
}

// NewHealthChecker builds a checker over the given resolver.
func NewHealthChecker(rv *Resolver) *HealthChecker {
	return &HealthChecker{rv: rv, version: probeVersion}
}

// Check resolves a tool and reports its health. Artifact-only tools (no runnable
// executable) report installed/location without a version.
func (h *HealthChecker) Check(t Tool) HealthStatus {
	s := HealthStatus{ID: t.ID, Name: t.Name, Category: t.Category}
	r, ok := h.rv.Resolve(t.ID)
	if !ok {
		s.Source = SourceMissing
		s.RepairHint = repairHint(t)
		return s
	}
	s.Installed = true
	s.Location = r.Path
	s.Source = r.Source
	if t.Executable != "" {
		args := t.Health.VersionArgs
		if args == nil {
			args = []string{"--version"}
		}
		s.Version = h.version(r.Path, args)
	}
	return s
}

// CheckAll returns the health of the given tool IDs, skipping unknown IDs.
func (h *HealthChecker) CheckAll(ids []string) []HealthStatus {
	out := make([]HealthStatus, 0, len(ids))
	for _, id := range ids {
		if t, ok := h.rv.reg.Get(id); ok {
			out = append(out, h.Check(t))
		}
	}
	return out
}

// repairHint suggests how to make a missing tool available.
func repairHint(t Tool) string {
	if t.Method == MethodDetect {
		return t.Hint
	}
	switch t.Location.Kind {
	case LocCore:
		return "run `karya install` (or `karya profile install core`)"
	case LocDocs:
		return "run `karya profile install docs`"
	case LocLang:
		return "run `karya lang add " + t.Location.Lang + "`"
	default:
		return "run `karya install`"
	}
}

// probeVersion runs the tool and returns the trimmed first output line, or ""
// when the command fails or prints nothing.
func probeVersion(path string, args []string) string {
	out, err := exec.Command(path, args...).Output() //nolint:gosec // resolved managed tool path
	if err != nil || len(out) == 0 {
		return ""
	}
	line, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	return strings.TrimSpace(line)
}
