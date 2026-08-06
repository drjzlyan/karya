//go:build integration

package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// execRunner is a minimal real-command runner for the integration test, kept
// local so the package has no test-time dependency on ship.
type execRunner struct{}

func (execRunner) Output(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}

func (execRunner) Run(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.Run()
}

// TestAddRemoveIsolatesUnderKaryaRoot drives real git to prove the core
// isolation promise: a task's worktree is checked out under the karya-owned root
// (never inside the user's repo), on a namespaced task/<id> branch, and Remove
// leaves nothing behind — no checkout dir and no branch.
func TestAddRemoveIsolatesUnderKaryaRoot(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	repo := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	git("init")
	git("config", "user.email", "t@example.com")
	git("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(repo, "README"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-m", "init")

	root := t.TempDir()
	m := Manager{Runner: execRunner{}, Root: root}

	path, err := m.Add(repo, "t1")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	// Isolation: the checkout lives under the karya root, not inside the repo.
	if !strings.HasPrefix(path, root) {
		t.Errorf("checkout %q not under karya root %q", path, root)
	}
	if strings.HasPrefix(path, repo) {
		t.Errorf("checkout %q leaked inside the user repo %q", path, repo)
	}
	// The repo's committed file is present in the isolated worktree.
	if _, err := os.Stat(filepath.Join(path, "README")); err != nil {
		t.Errorf("worktree missing repo content: %v", err)
	}
	// The branch is namespaced.
	if out, _ := (execRunner{}).Output(repo, "git", "branch", "--list", "task/t1"); !strings.Contains(out, "task/t1") {
		t.Errorf("branch task/t1 not created; got %q", out)
	}

	if err := m.Remove(repo, "t1"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	// Nothing left behind: neither the checkout dir nor the branch.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("checkout dir still present after Remove: %v", err)
	}
	if out, _ := (execRunner{}).Output(repo, "git", "branch", "--list", "task/t1"); strings.TrimSpace(out) != "" {
		t.Errorf("branch task/t1 survived Remove; got %q", out)
	}
}

// TestAddFromBranchesOffBaseRef proves base-ref selection: a task started from
// an older commit does not see later work on the user's branch, and dirty
// uncommitted content in the user's tree never leaks into the task worktree.
func TestAddFromBranchesOffBaseRef(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	repo := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return string(out)
	}
	git("init")
	git("config", "user.email", "t@example.com")
	git("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(repo, "f.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-m", "base")
	base := strings.TrimSpace(git("rev-parse", "HEAD"))

	// Later committed work plus a dirty, uncommitted change in the user's tree.
	if err := os.WriteFile(filepath.Join(repo, "f.txt"), []byte("newer\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-m", "newer work")
	if err := os.WriteFile(filepath.Join(repo, "dirty.txt"), []byte("uncommitted\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := Manager{Runner: execRunner{}, Root: t.TempDir()}
	path, err := m.AddFrom(repo, "t2", base)
	if err != nil {
		t.Fatalf("AddFrom: %v", err)
	}
	defer func() { _ = m.Remove(repo, "t2") }()

	data, err := os.ReadFile(filepath.Join(path, "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "base\n" {
		t.Errorf("task worktree has %q, want the base ref content", data)
	}
	if _, err := os.Stat(filepath.Join(path, "dirty.txt")); !os.IsNotExist(err) {
		t.Error("uncommitted change from the user's tree leaked into the task worktree")
	}
}
