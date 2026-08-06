// Package review assembles a task's reviewable artifacts — spec, plan, diff, and
// verification evidence — into one value for the human review surface (the
// `karya review` CLI and internal/reviewview). Nothing merges without a human
// gate crossing; this is what the human reads to decide (DESIGN.md §6, §7).
package review

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/drjzlyan/karya/internal/gate"
	"github.com/drjzlyan/karya/internal/spec"
	"github.com/drjzlyan/karya/internal/task"
)

// Differ produces the diff a task branch introduces over its base.
type Differ interface {
	DiffRange(base, head string) (string, error)
}

// Review is the assembled set of artifacts for a task at its current gate.
type Review struct {
	Task     task.Task
	Spec     *spec.Spec
	Plan     string   // PLAN.md contents ("" if none yet)
	Diff     string   // base...branch diff ("" if not started/none)
	Evidence []string // VERIFY-*.md contents, in order
	Pending  gate.Pending
	HasGate  bool // whether the task awaits a human gate
}

// Assemble reads a task's artifacts from the store and, when the task has a
// branch, its diff from differ (may be nil to skip the diff).
func Assemble(store *task.Store, differ Differ, id string) (*Review, error) {
	t, err := store.Get(id)
	if err != nil {
		return nil, err
	}
	r := &Review{Task: t}
	if sp, err := store.Spec(id); err == nil {
		r.Spec = sp
	}
	r.Plan = readFile(filepath.Join(store.Dir(id), "PLAN.md"))
	r.Evidence = readEvidence(store.Dir(id))
	if differ != nil && t.Base != "" && t.Branch != "" {
		if d, err := differ.DiffRange(t.Base, t.Branch); err == nil {
			r.Diff = d
		}
	}
	r.Pending, r.HasGate = gate.For(t.State)
	return r, nil
}

// readFile returns a file's contents or "" if it does not exist.
func readFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

// readEvidence returns the contents of every VERIFY-*.md in dir, sorted by name.
func readEvidence(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "VERIFY-") && strings.HasSuffix(e.Name(), ".md") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var out []string
	for _, n := range names {
		out = append(out, readFile(filepath.Join(dir, n)))
	}
	return out
}
