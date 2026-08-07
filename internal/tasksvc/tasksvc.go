// Package tasksvc holds karya's task lifecycle operations as reusable, in-process
// functions (DESIGN.md §2, §5). The TUI drives the gate lifecycle by calling
// these directly — plan/implement/verify/merge and the gate crossings all run in
// the karya process rather than shelling out to a `karya` subcommand. Each
// function takes an explicit Env (the repo, its task store, and the worktree
// manager) so it is decoupled from any CLI bootstrap and unit-testable.
package tasksvc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/drjzlyan/karya/internal/agentrun"
	"github.com/drjzlyan/karya/internal/config"
	"github.com/drjzlyan/karya/internal/gate"
	"github.com/drjzlyan/karya/internal/git"
	"github.com/drjzlyan/karya/internal/spec"
	"github.com/drjzlyan/karya/internal/task"
	"github.com/drjzlyan/karya/internal/verify"
	"github.com/drjzlyan/karya/internal/worktree"
)

// Env is the resolved repository context a caller supplies once: the repo root,
// its task store, and the worktree manager rooted at karya's state dir.
type Env struct {
	Repo      string
	Store     *task.Store
	Worktrees worktree.Manager
}

// RepoEnv resolves the git repository containing dir and builds its Env. Tasks
// live in the repo (.karya/tasks/); worktrees live under the karya-owned state
// dir so agent checkouts never pollute the user's tree.
func RepoEnv(dir string) (Env, error) {
	runner := agentrun.ExecRunner{}
	top, err := runner.Output(dir, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return Env{}, fmt.Errorf("not inside a git repository: %w", err)
	}
	top = strings.TrimSpace(top)
	mgr := worktree.Manager{Runner: runner, Root: config.Resolve().WorktreesDir()}
	return Env{Repo: top, Store: task.NewStore(top), Worktrees: mgr}, nil
}

// List returns the repo's tasks.
func List(e Env) ([]task.Task, error) { return e.Store.List() }

// NewTask scaffolds a draft task from slug with an optional preferred agent.
func NewTask(e Env, slug, agent string) (task.Task, error) {
	if !spec.ValidID(slug) {
		return task.Task{}, fmt.Errorf("slug %q must be lowercase alphanumerics and hyphens", slug)
	}
	id := task.NewID(slug, time.Now())
	return e.Store.Create(id, spec.Template(id), agent)
}

// Start gives a draft task its isolated environment: a git worktree on branch
// task/<id> forked from base (default HEAD), recorded in STATE.json.
func Start(e Env, id, base string) (task.Task, error) {
	if base == "" {
		base = "HEAD"
	}
	t, err := e.Store.Get(id)
	if err != nil {
		return task.Task{}, err
	}
	if t.Worktree != "" {
		return task.Task{}, fmt.Errorf("task %s already has a worktree at %s", t.ID, t.Worktree)
	}
	if t.State != task.StateDraft {
		return task.Task{}, fmt.Errorf("task %s is %s; only draft tasks can be started", t.ID, t.State)
	}
	g := agentrun.Git{Runner: agentrun.ExecRunner{}, Dir: e.Repo}
	sha, err := g.RevParse(base)
	if err != nil {
		return task.Task{}, fmt.Errorf("base ref %q does not resolve: %w", base, err)
	}
	path, err := e.Worktrees.AddFrom(e.Repo, t.ID, base)
	if err != nil {
		return task.Task{}, err
	}
	t.Base, t.Branch, t.Worktree = sha, worktree.Branch(t.ID), path
	if err := e.Store.Save(t); err != nil {
		_ = e.Worktrees.Remove(e.Repo, t.ID) // never leak a checkout on a failed save
		return task.Task{}, err
	}
	return t, nil
}

// Plan runs the plan step headlessly (the agent studies the repo and writes
// PLAN.md) and moves a draft task to PLANNED so it awaits the plan gate.
func Plan(ctx context.Context, e Env, id string) (agentrun.Outcome, error) {
	t, sp, err := e.taskWithSpec(id)
	if err != nil {
		return agentrun.Outcome{}, err
	}
	out, err := agentrun.RunStep(ctx, agentrun.Request{
		Step: agentrun.StepPlan, TaskID: id, Worktree: t.Worktree,
		TaskDir: e.Store.Dir(id), RepoRoot: e.Repo, Spec: sp, Feedback: latestFeedback(t),
	})
	if err != nil {
		return out, err
	}
	if t.State == task.StateDraft {
		if err := t.Transition(task.StatePlanned, "agent:"+out.Agent, ""); err != nil {
			return out, err
		}
		if err := e.Store.Save(t); err != nil {
			return out, err
		}
	}
	return out, nil
}

// Implement runs the implement step in the task worktree after the plan gate,
// then moves the task to IMPLEMENTING to await the diff gate.
func Implement(ctx context.Context, e Env, id string) (agentrun.Outcome, error) {
	t, sp, err := e.taskWithSpec(id)
	if err != nil {
		return agentrun.Outcome{}, err
	}
	if t.State != task.StateApproved {
		return agentrun.Outcome{}, fmt.Errorf("task %s must pass the plan gate first (state: %s)", id, t.State)
	}
	plan, _ := os.ReadFile(filepath.Join(e.Store.Dir(id), "PLAN.md"))
	out, err := agentrun.RunStep(ctx, agentrun.Request{
		Step: agentrun.StepImplement, TaskID: id, Worktree: t.Worktree,
		TaskDir: e.Store.Dir(id), RepoRoot: e.Repo, Spec: sp, Plan: string(plan),
	})
	if err != nil {
		return out, err
	}
	if err := t.Transition(task.StateImplementing, "agent:"+out.Agent, ""); err != nil {
		return out, err
	}
	if err := e.Store.Save(t); err != nil {
		return out, err
	}
	return out, nil
}

// Verify runs the spec's Verification commands in the task worktree and records
// the result as VERIFY-<n>.md evidence. It does not cross the verify gate — the
// human does that after reading the evidence.
func Verify(e Env, id string) (verify.Result, string, error) {
	t, sp, err := e.taskWithSpec(id)
	if err != nil {
		return verify.Result{}, "", err
	}
	if len(sp.Verification) == 0 {
		return verify.Result{}, "", errors.New("spec has no Verification commands")
	}
	res := verify.Run(t.Worktree, sp.Verification, verify.ShellRunner{})
	path, werr := writeEvidence(e.Store.Dir(id), res.Markdown())
	return res, path, werr
}

// Merge, after the verify gate, merges the task branch into the current branch
// (or opens a PR), moves the task to DONE, and tears down its worktree. Agents
// never merge; only this post-gate crossing does (DESIGN.md §4).
func Merge(e Env, id string, asPR bool) error {
	t, err := e.Store.Get(id)
	if err != nil {
		return err
	}
	if t.State != task.StateMerging {
		return fmt.Errorf("task %s must pass the verify gate first (state: %s)", id, t.State)
	}
	branch := t.Branch
	if branch == "" {
		branch = worktree.Branch(id)
	}
	if asPR {
		if err := openPR(e.Repo, branch); err != nil {
			return err
		}
	} else if err := git.New(e.Repo, nil).Merge(branch, "karya: merge task "+id); err != nil {
		return err
	}
	if err := t.Transition(task.StateDone, "human", ""); err != nil {
		return err
	}
	if err := e.Store.Save(t); err != nil {
		return err
	}
	_ = e.Worktrees.Remove(e.Repo, id) // best-effort teardown
	return nil
}

// Abandon tears down a task's worktree and branch and deletes its directory.
func Abandon(e Env, id string) error {
	t, err := e.Store.Get(id)
	if err != nil {
		return err
	}
	_ = e.Worktrees.Remove(e.Repo, t.ID)
	return e.Store.Delete(t.ID)
}

// CrossGate performs a human (or delegated) gate crossing over the task store.
// A rejection requires feedback.
func CrossGate(e Env, id, actor string, reject bool, feedback string) error {
	t, err := e.Store.Get(id)
	if err != nil {
		return err
	}
	p, ok := gate.For(t.State)
	if !ok {
		return fmt.Errorf("task %s is not awaiting a gate (state: %s)", id, t.State)
	}
	target := p.Approve
	if reject {
		target = p.Reject
		if feedback == "" {
			return errors.New("reject requires feedback")
		}
	}
	if err := t.Transition(target, actor, feedback); err != nil {
		return err
	}
	return e.Store.Save(t)
}

// taskWithSpec loads a task and its spec, requiring a worktree (the agent's cwd).
func (e Env) taskWithSpec(id string) (task.Task, *spec.Spec, error) {
	t, err := e.Store.Get(id)
	if err != nil {
		return task.Task{}, nil, err
	}
	if t.Worktree == "" {
		return task.Task{}, nil, fmt.Errorf("task %s has no worktree — start it first", id)
	}
	sp, err := e.Store.Spec(id)
	if err != nil {
		return task.Task{}, nil, err
	}
	return t, sp, nil
}

// latestFeedback returns the feedback from the task's most recent rejection, so a
// re-run revises against it (empty otherwise).
func latestFeedback(t task.Task) string {
	for i := len(t.History) - 1; i >= 0; i-- {
		if t.History[i].Feedback != "" {
			return t.History[i].Feedback
		}
	}
	return ""
}

// writeEvidence writes report to the next VERIFY-<n>.md in dir and returns its
// path.
func writeEvidence(dir, report string) (string, error) {
	n := 1
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "VERIFY-") && strings.HasSuffix(e.Name(), ".md") {
				n++
			}
		}
	}
	path := filepath.Join(dir, fmt.Sprintf("VERIFY-%d.md", n))
	return path, os.WriteFile(path, []byte(report), 0o644)
}

// openPR pushes the task branch and opens a PR with gh (best-effort).
func openPR(repo, branch string) error {
	r := git.ExecRunner{}
	if err := r.Run(repo, "git", "push", "-u", "origin", branch); err != nil {
		return fmt.Errorf("push %s: %w", branch, err)
	}
	if err := r.Run(repo, "gh", "pr", "create", "--fill", "--head", branch); err != nil {
		return fmt.Errorf("gh pr create (is gh installed and authed?): %w", err)
	}
	return nil
}
