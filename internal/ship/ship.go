// Package ship implements karya's agent-driven git flow: stage the working
// tree, have the active coding agent author a Conventional-Commit message,
// commit, and optionally push and open a pull request.
//
// The git plumbing here is deterministic and side-effect-isolated behind the
// Runner interface (dependency inversion, per AGENT.md) so it is unit-testable
// with a fake. Agent involvement is confined to authoring the message string;
// everything that mutates the repository is plain, reviewable git.
package ship

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Runner executes external commands in a working directory. Output captures
// stdout for inspection (status, diff); Run inherits the caller's stdio for
// interactive/streaming commands (commit, push). Defined here so callers can
// substitute a fake in tests.
type Runner interface {
	Output(dir, name string, args ...string) (string, error)
	Run(dir, name string, args ...string) error
}

// Git runs git operations for a single repository through a Runner.
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
// karya records a task's base commit at creation so a review can diff the whole
// task branch against exactly where it started.
func (g Git) RevParse(rev string) (string, error) {
	out, err := g.Runner.Output(g.Dir, "git", "rev-parse", rev)
	return strings.TrimSpace(out), err
}

// CommitAll stages every change in Dir and commits it with message, returning
// whether a commit was made (false when there was nothing to commit). It is how
// karya turns an agent's in-progress worktree edits into a reviewable commit —
// for a checkpoint or just before a merge.
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

// ResetHard resets Dir's working tree and index to ref, discarding changes after
// it. It backs `karya task rewind`, restoring a worktree to an earlier checkpoint.
func (g Git) ResetHard(ref string) error {
	return g.Runner.Run(g.Dir, "git", "reset", "--hard", ref)
}

// Merge merges branch into Dir's current branch. noFF forces a merge commit
// (--no-ff) so a task's history stays a distinct, revertible unit; otherwise git
// may fast-forward. It backs the diff-review "apply" step (`karya task merge`).
func (g Git) Merge(branch string, noFF bool) error {
	args := []string{"merge"}
	if noFF {
		args = append(args, "--no-ff")
	}
	args = append(args, branch)
	return g.Runner.Run(g.Dir, "git", args...)
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

// Prompt is the instruction handed to the agent to author a commit message from
// a staged diff. Kept small and explicit so any agent — headless or in-pane —
// produces a clean Conventional-Commit message and nothing else.
const promptPreamble = `Write a single Conventional Commits message for the staged changes below.
Rules: one short imperative subject line (<=72 chars), a blank line, then a concise body explaining the why.
Respond with ONLY the commit message — no code fences, no preamble, no trailing commentary.

Staged diff:
`

// BuildPrompt renders the message-authoring prompt for a staged diff.
func BuildPrompt(diff string) string {
	return promptPreamble + diff
}

// SanitizeMessage cleans an agent's reply into a usable commit message: it strips
// surrounding Markdown code fences, drops a leading conversational line when the
// agent prefixed one, and trims blank padding. It never invents content — an
// empty or fence-only reply yields "".
func SanitizeMessage(raw string) string {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")

	// Drop leading blank lines and a leading ``` fence.
	for len(lines) > 0 && (strings.TrimSpace(lines[0]) == "" || strings.HasPrefix(strings.TrimSpace(lines[0]), "```")) {
		lines = lines[1:]
	}
	// Drop a trailing fence and trailing blank lines.
	for len(lines) > 0 {
		last := strings.TrimSpace(lines[len(lines)-1])
		if last == "" || strings.HasPrefix(last, "```") {
			lines = lines[:len(lines)-1]
			continue
		}
		break
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// Subject returns the first line of a commit message, for use as a PR title.
func Subject(message string) string {
	if i := strings.IndexByte(message, '\n'); i >= 0 {
		return strings.TrimSpace(message[:i])
	}
	return strings.TrimSpace(message)
}
