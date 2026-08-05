package worktree

import (
	"reflect"
	"strings"
	"testing"
)

// fakeRunner records git invocations and answers rev-parse with a canned
// top-level so the unit tests stay hermetic (no real git).
type fakeRunner struct {
	toplevel string
	runs     [][]string
}

func (f *fakeRunner) Output(_, name string, args ...string) (string, error) {
	if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
		return f.toplevel + "\n", nil
	}
	return "", nil
}

func (f *fakeRunner) Run(_, name string, args ...string) error {
	f.runs = append(f.runs, append([]string{name}, args...))
	return nil
}

func TestBranchIsNamespaced(t *testing.T) {
	if got := Branch("abc"); got != "karya/abc" {
		t.Errorf("Branch = %q, want karya/abc", got)
	}
}

func TestPathUnderRootAndPerProject(t *testing.T) {
	m := Manager{Root: "/state/worktrees"}
	p1 := m.Path("/home/u/proj", "t1")
	if !strings.HasPrefix(p1, "/state/worktrees/") || !strings.HasSuffix(p1, "/t1") {
		t.Errorf("Path = %q, want under root ending /t1", p1)
	}
	// Same base name, different repo path → different slug (no collision).
	if p2 := m.Path("/other/proj", "t1"); slug(p1) == slug(p2) {
		t.Errorf("distinct repos shared a slug: %q vs %q", p1, p2)
	}
}

func slug(path string) string {
	// the segment between root and the trailing /<id>
	parts := strings.Split(strings.TrimPrefix(path, "/state/worktrees/"), "/")
	return parts[0]
}

func TestAddIssuesNamespacedWorktreeAdd(t *testing.T) {
	f := &fakeRunner{toplevel: t.TempDir()}
	m := Manager{Runner: f, Root: t.TempDir()}

	path, err := m.Add("/anywhere/in/repo", "task9")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	want := []string{"git", "worktree", "add", "-b", "karya/task9", path}
	if got := lastRun(f, "worktree"); !reflect.DeepEqual(got, want) {
		t.Errorf("worktree add call = %v, want %v", got, want)
	}
	if !strings.HasPrefix(path, m.Root) {
		t.Errorf("checkout path %q not under karya root %q", path, m.Root)
	}
}

func TestRemoveDeletesWorktreeAndBranch(t *testing.T) {
	f := &fakeRunner{toplevel: t.TempDir()}
	m := Manager{Runner: f, Root: t.TempDir()}
	if err := m.Remove("/anywhere", "task9"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if got := lastRun(f, "branch"); !reflect.DeepEqual(got, []string{"git", "branch", "-D", "karya/task9"}) {
		t.Errorf("branch delete call = %v", got)
	}
	if lastRun(f, "prune") == nil {
		t.Error("Remove did not prune stale worktree entries")
	}
}

// lastRun returns the most recent git call whose args contain marker.
func lastRun(f *fakeRunner, marker string) []string {
	for i := len(f.runs) - 1; i >= 0; i-- {
		for _, a := range f.runs[i] {
			if a == marker {
				return f.runs[i]
			}
		}
	}
	return nil
}
