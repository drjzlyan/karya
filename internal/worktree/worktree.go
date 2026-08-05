// Package worktree gives each karya task an isolated git worktree. Task-level
// isolation is the new primitive behind karya's human-in-the-loop, agents-first
// model: an agent works on a namespaced `karya/<task-id>` branch checked out
// under a karya-owned root (config.Paths.WorktreesDir), never in the user's own
// working tree, so its changes are contained and reviewable before they are
// merged. This extends karya's environment-isolation guarantee (PLAN.md §2) to
// the task level.
package worktree

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Runner executes git commands in a directory. It mirrors ship.Runner so the
// production ship.ExecRunner satisfies it structurally; it is defined here, on
// the consumer side, to keep this package free of any dependency on ship.
type Runner interface {
	Output(dir, name string, args ...string) (string, error)
	Run(dir, name string, args ...string) error
}

// BranchPrefix namespaces every karya task branch so they never collide with a
// user's own branches and are trivially identifiable for cleanup.
const BranchPrefix = "karya/"

// Branch returns the namespaced branch name for a task id.
func Branch(id string) string { return BranchPrefix + id }

// Manager creates and removes per-task worktrees under a karya-owned root.
type Manager struct {
	Runner Runner
	Root   string // config.Paths.WorktreesDir()
}

// Path returns the checkout directory for task id in repoDir, without creating
// it. Checkouts are grouped by a per-project slug so tasks from different repos
// never share a directory.
func (m Manager) Path(repoDir, id string) string {
	return filepath.Join(m.Root, ProjectSlug(repoDir), id)
}

// Add creates an isolated worktree for task id: a new branch karya/<id> off the
// repository's current HEAD, checked out under the karya-owned root. It returns
// the checkout path. The repository must have at least one commit (HEAD must
// resolve) — git worktree cannot branch from an unborn HEAD.
func (m Manager) Add(repoDir, id string) (string, error) {
	top, err := m.topLevel(repoDir)
	if err != nil {
		return "", err
	}
	dst := m.Path(top, id)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", fmt.Errorf("worktree: create root: %w", err)
	}
	if err := m.Runner.Run(top, "git", "worktree", "add", "-b", Branch(id), dst); err != nil {
		return "", fmt.Errorf("worktree add %s: %w", id, err)
	}
	return dst, nil
}

// Remove force-removes the task's worktree and deletes its branch. Each step is
// best-effort so a partially-created task still cleans up; it prunes stale
// administrative entries and any residual checkout directory at the end. After
// Remove, karya has left nothing behind for the task in either the karya root or
// the repository's branch list.
func (m Manager) Remove(repoDir, id string) error {
	top, err := m.topLevel(repoDir)
	if err != nil {
		top = repoDir // best-effort: still try to clear the checkout dir below
	}
	dst := m.Path(top, id)
	// --force so a worktree with uncommitted changes (a rejected review) still
	// removes cleanly.
	_ = m.Runner.Run(top, "git", "worktree", "remove", "--force", dst)
	_ = m.Runner.Run(top, "git", "branch", "-D", Branch(id))
	_ = m.Runner.Run(top, "git", "worktree", "prune")
	if err := os.RemoveAll(dst); err != nil {
		return fmt.Errorf("worktree: remove dir %s: %w", dst, err)
	}
	return nil
}

// topLevel resolves repoDir to the repository's top-level directory so worktrees
// are always anchored to the repo root regardless of the caller's subdirectory.
func (m Manager) topLevel(repoDir string) (string, error) {
	out, err := m.Runner.Output(repoDir, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("worktree: %q is not a git repository: %w", repoDir, err)
	}
	return strings.TrimSpace(out), nil
}

// ProjectSlug derives a stable, readable, collision-resistant name for a
// repository from its absolute path: the base name plus a short hash, so two
// projects that share a base name never collide. karya reuses it both for the
// worktree root and for the per-project task store filename, keeping the two in
// lockstep for one project.
func ProjectSlug(repoDir string) string {
	sum := sha256.Sum256([]byte(repoDir))
	return sanitize(filepath.Base(repoDir)) + "-" + hex.EncodeToString(sum[:])[:8]
}

// sanitize makes a string safe as a single path segment.
func sanitize(s string) string {
	return strings.NewReplacer(string(filepath.Separator), "_", " ", "_", ":", "_").Replace(s)
}
