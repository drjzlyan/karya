// Package git is karya's headless git service: the operations the built-in git
// panel (internal/gitui) and task review need — status, stage/unstage, commit,
// diff, log, branches, push — over an injectable Runner. It replaces the need to
// shell out to lazygit (DESIGN.md §6, §12). All logic lives here, below the UI,
// so it is tested against real git in a temp repo and with a fake Runner for
// parsing, keeping the panel view thin.
package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// Runner executes git in a working directory. Output captures stdout; Run
// streams stdio (for commands with hooks/prompts). It is an interface so tests
// can substitute a fake.
type Runner interface {
	Output(dir, name string, args ...string) (string, error)
	Run(dir, name string, args ...string) error
}

// ExecRunner is the production Runner shelling out to real commands.
type ExecRunner struct{}

// Output runs name in dir and returns trimmed stdout, folding stderr into the
// error on failure so git's own diagnostic surfaces.
func (ExecRunner) Output(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), msg)
		}
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// Run runs name in dir with stdio inherited.
func (ExecRunner) Run(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.Run()
}

// Repo is a git working tree at Dir, operated through a Runner.
type Repo struct {
	Dir string
	run Runner
}

// New returns a Repo at dir. A nil runner uses the production ExecRunner.
func New(dir string, r Runner) *Repo {
	if r == nil {
		r = ExecRunner{}
	}
	return &Repo{Dir: dir, run: r}
}

func (r *Repo) out(args ...string) (string, error) { return r.run.Output(r.Dir, "git", args...) }
func (r *Repo) do(args ...string) error            { return r.run.Run(r.Dir, "git", args...) }

// InsideRepo reports whether Dir is within a git work tree.
func (r *Repo) InsideRepo() bool {
	out, err := r.out("rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(out) == "true"
}

// FileStatus is one entry from `git status --porcelain`.
type FileStatus struct {
	Path      string
	Index     byte // staged status code (' ' when none)
	Work      byte // worktree status code (' ' when none)
	Untracked bool
}

// Staged reports whether the file has staged changes.
func (f FileStatus) Staged() bool { return !f.Untracked && f.Index != ' ' && f.Index != 0 }

// Unstaged reports whether the file has unstaged changes (incl. untracked).
func (f FileStatus) Unstaged() bool { return f.Untracked || (f.Work != ' ' && f.Work != 0) }

// Status returns the working tree status (porcelain v1). Renames report the new
// path.
func (r *Repo) Status() ([]FileStatus, error) {
	out, err := r.out("status", "--porcelain")
	if err != nil {
		return nil, err
	}
	return parseStatus(out), nil
}

func parseStatus(out string) []FileStatus {
	var files []FileStatus
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		index, work := line[0], line[1]
		path := line[3:]
		// Renames/copies: "R  old -> new" — take the new path.
		if i := strings.Index(path, " -> "); i >= 0 {
			path = path[i+4:]
		}
		files = append(files, FileStatus{
			Path:      path,
			Index:     index,
			Work:      work,
			Untracked: index == '?' && work == '?',
		})
	}
	return files
}

// Stage stages a path (git add).
func (r *Repo) Stage(path string) error { return r.do("add", "--", path) }

// Unstage unstages a path (git restore --staged), tolerating a repo with no
// commits yet by falling back to `git rm --cached`.
func (r *Repo) Unstage(path string) error {
	if err := r.do("restore", "--staged", "--", path); err != nil {
		return r.do("reset", "-q", "HEAD", "--", path)
	}
	return nil
}

// StageAll stages every change (git add -A).
func (r *Repo) StageAll() error { return r.do("add", "-A") }

// UnstageAll unstages everything.
func (r *Repo) UnstageAll() error {
	if err := r.do("restore", "--staged", "."); err != nil {
		return r.do("reset", "-q")
	}
	return nil
}

// Commit records staged changes with message.
func (r *Repo) Commit(message string) error { return r.do("commit", "-m", message) }

// Diff returns the unified diff for a path. When staged is true it shows the
// staged diff (--cached); otherwise the worktree diff. An empty path diffs
// everything.
func (r *Repo) Diff(path string, staged bool) (string, error) {
	args := []string{"diff"}
	if staged {
		args = append(args, "--cached")
	}
	if path != "" {
		args = append(args, "--", path)
	}
	return r.out(args...)
}

// DiffRange returns the unified diff of head relative to their merge base with
// base (git diff base...head) — the diff a task's branch introduces.
func (r *Repo) DiffRange(base, head string) (string, error) {
	return r.out("diff", base+"..."+head)
}

// Merge merges branch into the current branch with a merge commit.
func (r *Repo) Merge(branch, message string) error {
	return r.do("merge", "--no-ff", "-m", message, branch)
}

// Commit summary line.
type Commit struct {
	Hash    string
	Subject string
	Author  string
	When    string // relative committer date, e.g. "2 hours ago"
}

// Log returns up to n recent commits (newest first).
func (r *Repo) Log(n int) ([]Commit, error) {
	out, err := r.out("log", fmt.Sprintf("-%d", n), "--pretty=%h\x1f%s\x1f%an\x1f%cr")
	if err != nil {
		return nil, err
	}
	var commits []Commit
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x1f", 4)
		c := Commit{Hash: parts[0]}
		if len(parts) > 1 {
			c.Subject = parts[1]
		}
		if len(parts) > 2 {
			c.Author = parts[2]
		}
		if len(parts) > 3 {
			c.When = parts[3]
		}
		commits = append(commits, c)
	}
	return commits, nil
}

// Show returns the diff a commit introduced (git show), for previewing history
// in the git panel.
func (r *Repo) Show(ref string) (string, error) {
	return r.out("show", "--stat", "--patch", ref)
}

// CurrentBranch returns the checked-out branch name (or "HEAD" when detached).
func (r *Repo) CurrentBranch() (string, error) {
	return r.out("rev-parse", "--abbrev-ref", "HEAD")
}

// Branches lists local branch names.
func (r *Repo) Branches() ([]string, error) {
	out, err := r.out("branch", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			names = append(names, l)
		}
	}
	return names, nil
}

// Push pushes the current branch to its upstream (best-effort: sets upstream on
// first push).
func (r *Repo) Push() error {
	if err := r.do("push"); err != nil {
		branch, berr := r.CurrentBranch()
		if berr != nil {
			return err
		}
		return r.do("push", "-u", "origin", branch)
	}
	return nil
}
