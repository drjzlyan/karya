package agentrun

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Git runs git operations for a single repository through a Runner. The git
// plumbing behind task execution is deterministic and side-effect-isolated
// behind the Runner interface (dependency inversion, per AGENTS.md) so it is
// unit-testable with a fake. Agent involvement is confined to authoring content
// (plans, commit messages); everything that mutates the repository is plain,
// reviewable git.
type Git struct {
	Runner Runner
	Dir    string
}

// InsideRepo reports whether Dir is within a git work tree.
func (g Git) InsideRepo() bool {
	out, err := g.Runner.Output(g.Dir, "git", "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(out) == "true"
}

// StageAll stages every change in the work tree (git add -A).
func (g Git) StageAll() error {
	return g.Runner.Run(g.Dir, "git", "add", "-A")
}

// HasStaged reports whether anything is staged for commit.
func (g Git) HasStaged() (bool, error) {
	out, err := g.Runner.Output(g.Dir, "git", "diff", "--cached", "--name-only")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// StagedDiff returns the staged diff, used as context for message authoring.
func (g Git) StagedDiff() (string, error) {
	return g.Runner.Output(g.Dir, "git", "diff", "--cached")
}

// CurrentBranch returns the current branch name.
func (g Git) CurrentBranch() (string, error) {
	out, err := g.Runner.Output(g.Dir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	return strings.TrimSpace(out), err
}

// Commit records the staged changes with message. The message is passed via a
// temp file (-F) so multi-line Conventional-Commit bodies survive intact.
func (g Git) Commit(message string, noVerify bool) error {
	f, err := os.CreateTemp("", "karya-commit-*.txt")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(message); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	args := []string{"commit", "-F", f.Name()}
	if noVerify {
		args = append(args, "--no-verify")
	}
	return g.Runner.Run(g.Dir, "git", args...)
}

// RevParse resolves a revision (e.g. "HEAD", a branch, a tag) to its commit SHA.
// karya records a task's base commit at `task start` so the diff gate can later
// diff the task branch against exactly where it started.
func (g Git) RevParse(rev string) (string, error) {
	out, err := g.Runner.Output(g.Dir, "git", "rev-parse", rev)
	return strings.TrimSpace(out), err
}

// CommitAll stages every change in Dir and commits it with message, returning
// whether a commit was made (false when there was nothing to commit). It is how
// karya turns an agent's in-progress worktree edits into a reviewable commit
// ahead of the diff gate.
func (g Git) CommitAll(message string, noVerify bool) (bool, error) {
	if err := g.StageAll(); err != nil {
		return false, err
	}
	staged, err := g.HasStaged()
	if err != nil {
		return false, err
	}
	if !staged {
		return false, nil
	}
	if err := g.Commit(message, noVerify); err != nil {
		return false, err
	}
	return true, nil
}

// DiffCachedAgainst returns the diff of Dir's staged tree against base. Callers
// StageAll first so the review includes new (untracked) files as well as edits —
// the complete set of changes an agent made in its worktree since base.
func (g Git) DiffCachedAgainst(base string) (string, error) {
	return g.Runner.Output(g.Dir, "git", "diff", "--cached", base)
}

// ResetHard resets Dir's working tree and index to ref, discarding changes
// after it. Reserved for the task verify/merge flow (Phase D).
func (g Git) ResetHard(ref string) error {
	return g.Runner.Run(g.Dir, "git", "reset", "--hard", ref)
}

// Merge merges branch into Dir's current branch. noFF forces a merge commit
// (--no-ff) so a task's history stays a distinct, revertible unit; otherwise git
// may fast-forward. Reserved for the post-verify-gate merge (Phase D).
func (g Git) Merge(branch string, noFF bool) error {
	args := []string{"merge"}
	if noFF {
		args = append(args, "--no-ff")
	}
	args = append(args, branch)
	return g.Runner.Run(g.Dir, "git", args...)
}

// AbortMerge aborts an in-progress merge, restoring Dir to its pre-merge state.
// karya calls it when a task merge hits conflicts so the user's working tree is
// never left half-merged.
func (g Git) AbortMerge() error {
	return g.Runner.Run(g.Dir, "git", "merge", "--abort")
}

// Push pushes the current branch, setting upstream when it has none.
func (g Git) Push() error {
	branch, err := g.CurrentBranch()
	if err != nil || branch == "" {
		return g.Runner.Run(g.Dir, "git", "push")
	}
	return g.Runner.Run(g.Dir, "git", "push", "--set-upstream", "origin", branch)
}

// CreatePR opens a pull request with the GitHub CLI, filling title/body from the
// arguments. It is a no-op returning an error when gh is unavailable.
func (g Git) CreatePR(title, body string) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("gh (GitHub CLI) not found; skip --pr or install gh")
	}
	return g.Runner.Run(g.Dir, "gh", "pr", "create", "--title", title, "--body", body)
}
