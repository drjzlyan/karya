//go:build integration

package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// tempRepo initializes a git repo in a temp dir with deterministic identity.
func tempRepo(t *testing.T) *Repo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=k", "GIT_AUTHOR_EMAIL=k@k",
			"GIT_COMMITTER_NAME=k", "GIT_COMMITTER_EMAIL=k@k",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.name", "k")
	run("config", "user.email", "k@k")
	return New(dir, execWithEnv{})
}

// execWithEnv is an ExecRunner that injects a deterministic commit identity.
type execWithEnv struct{}

func (execWithEnv) Output(dir, name string, args ...string) (string, error) {
	return ExecRunner{}.Output(dir, name, args...)
}
func (execWithEnv) Run(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=k", "GIT_AUTHOR_EMAIL=k@k",
		"GIT_COMMITTER_NAME=k", "GIT_COMMITTER_EMAIL=k@k",
	)
	return cmd.Run()
}

func TestRepoLifecycle(t *testing.T) {
	r := tempRepo(t)
	if !r.InsideRepo() {
		t.Fatal("should be inside a repo")
	}
	// Create a file -> untracked.
	if err := os.WriteFile(filepath.Join(r.Dir, "a.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := r.Status()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || !files[0].Untracked {
		t.Fatalf("expected one untracked file, got %+v", files)
	}
	// Stage -> staged.
	if err := r.Stage("a.txt"); err != nil {
		t.Fatal(err)
	}
	files, _ = r.Status()
	if len(files) != 1 || !files[0].Staged() {
		t.Fatalf("expected staged, got %+v", files)
	}
	// Staged diff should mention the file.
	diff, err := r.Diff("a.txt", true)
	if err != nil {
		t.Fatal(err)
	}
	if diff == "" {
		t.Fatal("staged diff empty")
	}
	// Commit -> clean.
	if err := r.Commit("add a"); err != nil {
		t.Fatal(err)
	}
	files, _ = r.Status()
	if len(files) != 0 {
		t.Fatalf("expected clean tree, got %+v", files)
	}
	// Log has the commit.
	commits, err := r.Log(5)
	if err != nil || len(commits) != 1 || commits[0].Subject != "add a" {
		t.Fatalf("log = %+v, err %v", commits, err)
	}
	// Branch is main.
	if b, _ := r.CurrentBranch(); b != "main" {
		t.Fatalf("branch = %q want main", b)
	}
}

func TestUnstage(t *testing.T) {
	r := tempRepo(t)
	// initial commit so restore --staged has a HEAD
	os.WriteFile(filepath.Join(r.Dir, "base.txt"), []byte("x\n"), 0o644)
	r.Stage("base.txt")
	r.Commit("base")
	os.WriteFile(filepath.Join(r.Dir, "b.txt"), []byte("hi\n"), 0o644)
	r.Stage("b.txt")
	if f, _ := r.Status(); len(f) != 1 || !f[0].Staged() {
		t.Fatalf("expected staged b.txt, got %+v", f)
	}
	if err := r.Unstage("b.txt"); err != nil {
		t.Fatal(err)
	}
	if f, _ := r.Status(); len(f) != 1 || f[0].Staged() {
		t.Fatalf("expected unstaged after Unstage, got %+v", f)
	}
}
