package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/drjzlyan/karya/internal/agentrun"
	"github.com/drjzlyan/karya/internal/task"
)

// cmdPlan implements `karya plan <id>`: it runs the plan step headlessly in the
// task worktree (the agent studies the repo and writes PLAN.md), then moves the
// task to PLANNED so it awaits the human plan gate (DESIGN.md §2, §5). Re-running
// after a plan-gate rejection (state back to DRAFT) revises the plan.
func cmdPlan(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: karya plan <id>")
		return 2
	}
	a, err := newApp()
	if err != nil {
		return fail(err)
	}
	_, store, top, err := taskContext(a)
	if err != nil {
		return fail(err)
	}
	id := args[0]
	t, err := store.Get(id)
	if err != nil {
		return fail(err)
	}
	if t.Worktree == "" {
		fmt.Fprintf(os.Stderr, "task %s has no worktree — run `karya task start %s` first\n", id, id)
		return 2
	}
	sp, err := store.Spec(id)
	if err != nil {
		return fail(err)
	}

	fmt.Printf("Planning %s …\n", id)
	out, err := agentrun.RunStep(context.Background(), agentrun.Request{
		Step: agentrun.StepPlan, TaskID: id, Worktree: t.Worktree,
		TaskDir: store.Dir(id), RepoRoot: top, Spec: sp, Feedback: latestFeedback(t),
	})
	if err != nil {
		return fail(err)
	}
	if t.State == task.StateDraft {
		if err := t.Transition(task.StatePlanned, "agent:"+out.Agent, ""); err != nil {
			return fail(err)
		}
		if err := store.Save(t); err != nil {
			return fail(err)
		}
	}
	fmt.Printf("planned %s (agent %s) → %s\nreview: karya review %s\n", id, out.Agent, out.PlanPath, id)
	return 0
}

// cmdImplement implements `karya implement <id>`: after the plan gate (state
// APPROVED), the agent implements the approved plan in the task worktree, then
// the task moves to IMPLEMENTING to await the diff gate (DESIGN.md §2, §5).
func cmdImplement(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: karya implement <id>")
		return 2
	}
	a, err := newApp()
	if err != nil {
		return fail(err)
	}
	_, store, top, err := taskContext(a)
	if err != nil {
		return fail(err)
	}
	id := args[0]
	t, err := store.Get(id)
	if err != nil {
		return fail(err)
	}
	if t.Worktree == "" {
		fmt.Fprintf(os.Stderr, "task %s has no worktree — run `karya task start %s` first\n", id, id)
		return 2
	}
	if t.State != task.StateApproved {
		fmt.Fprintf(os.Stderr, "task %s must pass the plan gate first (state: %s)\n", id, t.State)
		return 2
	}
	sp, err := store.Spec(id)
	if err != nil {
		return fail(err)
	}
	plan, _ := os.ReadFile(filepath.Join(store.Dir(id), "PLAN.md"))

	fmt.Printf("Implementing %s …\n", id)
	out, err := agentrun.RunStep(context.Background(), agentrun.Request{
		Step: agentrun.StepImplement, TaskID: id, Worktree: t.Worktree,
		TaskDir: store.Dir(id), RepoRoot: top, Spec: sp, Plan: string(plan),
	})
	if err != nil {
		return fail(err)
	}
	if err := t.Transition(task.StateImplementing, "agent:"+out.Agent, ""); err != nil {
		return fail(err)
	}
	if err := store.Save(t); err != nil {
		return fail(err)
	}
	fmt.Printf("implemented %s (agent %s)\nreview the diff: karya review %s\n", id, out.Agent, id)
	return 0
}

// latestFeedback returns the feedback from the task's most recent rejection, so
// a re-run revises against it (empty otherwise).
func latestFeedback(t task.Task) string {
	for i := len(t.History) - 1; i >= 0; i-- {
		if t.History[i].Feedback != "" {
			return t.History[i].Feedback
		}
	}
	return ""
}
