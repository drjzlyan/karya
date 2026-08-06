package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/drjzlyan/karya/internal/ship"
	"github.com/drjzlyan/karya/internal/spec"
	"github.com/drjzlyan/karya/internal/task"
	"github.com/drjzlyan/karya/internal/worktree"
)

// cmdTask dispatches `karya task <subcommand>`: the Phase A surface of the
// task engine (DESIGN.md §12). A task is created from a spec contract, gets an
// isolated git worktree on a task/<id> branch when started, and moves through
// the gate state machine recorded in STATE.json — the plan/implement/verify
// steps that drive agents arrive in Phases B–D.
func cmdTask(args []string) int {
	sub := "list"
	if len(args) > 0 {
		sub = args[0]
	}
	rest := args[1:]

	a, err := newApp()
	if err != nil {
		return fail(err)
	}

	switch sub {
	case "new":
		return cmdTaskNew(a, rest)
	case "list", "ls":
		return cmdTaskList(a)
	case "status":
		return cmdTaskStatus(a)
	case "show":
		return cmdTaskShow(a, rest)
	case "start":
		return cmdTaskStart(a, rest)
	case "abandon":
		return cmdTaskAbandon(a, rest)
	default:
		fmt.Fprintf(os.Stderr, "karya task: unknown subcommand %q\n", sub)
		fmt.Fprintln(os.Stderr, "usage: karya task new <slug> | list | status | show <id> | start <id> | abandon <id>")
		return 2
	}
}

// taskContext resolves the git repository the caller is in and returns the
// worktree Manager and the repository's task Store bound to it. Tasks live in
// the repo (.karya/tasks/), worktrees under the karya-owned state dir. The
// caller's cwd wins when it is inside a git repository; the session's project
// dir is only the fallback (a pane cd'd outside the project must not scatter
// tasks into the session's repo).
func taskContext(a *app) (worktree.Manager, *task.Store, string, error) {
	runner := ship.ExecRunner{}
	cwd, _ := os.Getwd()
	top, err := runner.Output(cwd, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		if dir := sessionWorkdir(a); dir != "" && dir != cwd {
			top, err = runner.Output(dir, "git", "rev-parse", "--show-toplevel")
		}
		if err != nil {
			return worktree.Manager{}, nil, "", fmt.Errorf("karya task must run inside a git repository: %w", err)
		}
	}
	top = strings.TrimSpace(top)
	mgr := worktree.Manager{Runner: runner, Root: a.paths.WorktreesDir()}
	return mgr, task.NewStore(top), top, nil
}

// cmdTaskNew implements `karya task new <slug> [--agent <name>]`: it scaffolds
// a spec contract at .karya/tasks/<date>-<slug>/SPEC.md and records the task at
// draft. The human fills in the spec (karya opens it in the editor pane when
// inside a session), then starts the task.
func cmdTaskNew(a *app, args []string) int {
	fs := flag.NewFlagSet("task new", flag.ContinueOnError)
	agentFlag := fs.String("agent", "", "preferred agent for implementation")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	slug := strings.Join(fs.Args(), "-")
	if slug == "" {
		fmt.Fprintln(os.Stderr, "usage: karya task new <slug> [--agent <name>]")
		return 2
	}
	if !spec.ValidID(slug) {
		return fail(fmt.Errorf("slug %q must be lowercase alphanumerics and hyphens", slug))
	}

	_, store, _, err := taskContext(a)
	if err != nil {
		return fail(err)
	}

	id := task.NewID(slug, time.Now())
	t, err := store.Create(id, spec.Template(id), *agentFlag)
	if err != nil {
		return fail(err)
	}

	specPath := store.SpecPath(id)
	fmt.Printf("Created task %s (draft)\n  spec: %s\n", t.ID, specPath)

	// Inside a karya session, open the spec in the editor pane so the human can
	// fill in the contract right away; standalone, point at the file.
	if os.Getenv("TMUX") != "" {
		return cmdEdit([]string{specPath})
	}
	fmt.Printf("\nFill in the spec, then: karya task start %s\n", t.ID)
	return 0
}

// cmdTaskList implements `karya task list`: the task board — every task with
// its state, agent, and the first line of its spec objective.
func cmdTaskList(a *app) int {
	_, store, _, err := taskContext(a)
	if err != nil {
		return fail(err)
	}
	tasks, err := store.List()
	if err != nil {
		return fail(err)
	}
	if len(tasks) == 0 {
		fmt.Println("No tasks yet. Create one with: karya task new <slug>")
		return 0
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	renderTaskList(w, tasks, titles(store, tasks))
	_ = w.Flush()
	return 0
}

// titles maps task ids to their spec-derived titles (best-effort: a task with
// an unparsable spec still lists, title-less).
func titles(store *task.Store, tasks []task.Task) map[string]string {
	out := map[string]string{}
	for _, t := range tasks {
		if s, err := store.Spec(t.ID); err == nil {
			out[t.ID] = task.Title(s)
		}
	}
	return out
}

// renderTaskList writes the task board table. Pure given its inputs so the
// layout is unit-testable.
func renderTaskList(w *tabwriter.Writer, tasks []task.Task, titles map[string]string) {
	fmt.Fprintln(w, "ID\tSTATE\tAGENT\tUPDATED\tTITLE")
	for _, t := range tasks {
		agentName := t.Agent
		if agentName == "" {
			agentName = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			t.ID, t.State, agentName, t.Updated.Local().Format("2006-01-02 15:04"), titles[t.ID])
	}
}

// cmdTaskStatus implements `karya task status`: per-state counts plus the gate
// inbox — tasks parked at a state whose forward move needs a human approval.
func cmdTaskStatus(a *app) int {
	_, store, _, err := taskContext(a)
	if err != nil {
		return fail(err)
	}
	tasks, err := store.List()
	if err != nil {
		return fail(err)
	}
	sum := task.Summarize(tasks)
	fmt.Printf("%d task(s)\n", sum.Total)
	for _, st := range []task.State{
		task.StateDraft, task.StatePlanned, task.StateApproved, task.StateImplementing,
		task.StateVerifying, task.StateMerging, task.StateDone, task.StateAbandoned,
	} {
		if n := sum.Counts[st]; n > 0 {
			fmt.Printf("  %-13s %d\n", st, n)
		}
	}
	if len(sum.Pending) > 0 {
		fmt.Println("\nAwaiting a human gate:")
		for _, t := range sum.Pending {
			fmt.Printf("  %s (%s)\n", t.ID, t.State)
		}
	}
	return 0
}

// cmdTaskShow implements `karya task show <id>`: the task's state, workspace,
// spec summary, and full gate history (the audit trail).
func cmdTaskShow(a *app, args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: karya task show <id>")
		return 2
	}
	_, store, _, err := taskContext(a)
	if err != nil {
		return fail(err)
	}
	t, err := store.Get(args[0])
	if err != nil {
		return fail(err)
	}
	fmt.Printf("task:     %s\nstate:    %s\n", t.ID, t.State)
	if t.Agent != "" {
		fmt.Printf("agent:    %s\n", t.Agent)
	}
	if t.Branch != "" {
		fmt.Printf("branch:   %s\n", t.Branch)
	}
	if t.Worktree != "" {
		fmt.Printf("worktree: %s\n", t.Worktree)
	}
	if t.Base != "" {
		fmt.Printf("base:     %s\n", short(t.Base))
	}
	if s, err := store.Spec(t.ID); err == nil {
		fmt.Printf("\nObjective: %s\n", task.Title(s))
		fmt.Printf("Acceptance criteria: %d (%d checked)\n", len(s.Criteria), checked(s.Criteria))
		fmt.Printf("Verification commands: %d\n", len(s.Verification))
	}
	if len(t.History) > 0 {
		fmt.Println("\nHistory:")
		for _, h := range t.History {
			line := fmt.Sprintf("  %s  %s → %s", h.At.Local().Format("2006-01-02 15:04"), h.From, h.To)
			if h.Gate != "" {
				line += fmt.Sprintf(" (gate: %s, by %s)", h.Gate, h.Actor)
			} else {
				line += fmt.Sprintf(" (by %s)", h.Actor)
			}
			fmt.Println(line)
			if h.Feedback != "" {
				fmt.Printf("    feedback: %s\n", h.Feedback)
			}
		}
	}
	return 0
}

// checked counts ticked acceptance criteria.
func checked(criteria []spec.Criterion) int {
	n := 0
	for _, c := range criteria {
		if c.Checked {
			n++
		}
	}
	return n
}

// cmdTaskStart implements `karya task start <id> [--base <ref>]`: it gives the
// task its isolated environment — a git worktree on branch task/<id> forked
// from the base ref (default HEAD) under the karya-owned root — and records it
// in STATE.json. The plan step that runs an agent in that worktree arrives in
// Phase B; until then the worktree is ready for interactive work.
func cmdTaskStart(a *app, args []string) int {
	fs := flag.NewFlagSet("task start", flag.ContinueOnError)
	base := fs.String("base", "HEAD", "base ref the task branch forks from")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: karya task start <id> [--base <ref>]")
		return 2
	}

	mgr, store, top, err := taskContext(a)
	if err != nil {
		return fail(err)
	}
	t, err := store.Get(fs.Arg(0))
	if err != nil {
		return fail(err)
	}
	if t.Worktree != "" {
		return fail(fmt.Errorf("task %s already has a worktree at %s", t.ID, t.Worktree))
	}
	if t.State != task.StateDraft {
		return fail(fmt.Errorf("task %s is %s; only draft tasks can be started", t.ID, t.State))
	}

	// Record the exact commit the task branches from so the diff gate can later
	// diff the task against precisely where it started.
	sha, err := gitAt(top).RevParse(*base)
	if err != nil {
		return fail(fmt.Errorf("base ref %q does not resolve: %w", *base, err))
	}
	path, err := mgr.AddFrom(top, t.ID, *base)
	if err != nil {
		return fail(err)
	}

	t.Base, t.Branch, t.Worktree = sha, worktree.Branch(t.ID), path
	if err := store.Save(t); err != nil {
		// Roll back the worktree so a failed save never leaks a checkout/branch.
		_ = mgr.Remove(top, t.ID)
		return fail(err)
	}

	fmt.Printf("Started task %s\n  branch:   %s (from %s)\n  worktree: %s\n",
		t.ID, t.Branch, short(sha), t.Worktree)
	return 0
}

// cmdTaskAbandon implements `karya task abandon <id> [-y]`: clean teardown —
// the worktree and task/<id> branch are removed and the task directory (spec,
// state, artifacts) is deleted (DESIGN.md §4). Merged tasks keep their
// artifacts via `task done` (Phase D) instead.
func cmdTaskAbandon(a *app, args []string) int {
	id, yes := parseIDYesArgs(args)
	if id == "" {
		fmt.Fprintln(os.Stderr, "usage: karya task abandon <id> [-y]")
		return 2
	}
	mgr, store, top, err := taskContext(a)
	if err != nil {
		return fail(err)
	}
	t, err := store.Get(id)
	if err != nil {
		return fail(err)
	}
	if !yes && !confirm(os.Stdin, fmt.Sprintf(
		"Abandon task %s? This removes its worktree, branch %s, and all artifacts", t.ID, worktree.Branch(t.ID))) {
		fmt.Println("Aborted.")
		return 1
	}
	// Best-effort teardown of the isolated environment; the store delete is the
	// source of truth for "the task is gone".
	_ = mgr.Remove(top, t.ID)
	if err := store.Delete(t.ID); err != nil {
		return fail(err)
	}
	fmt.Printf("Abandoned task %s (worktree, branch, and artifacts removed)\n", t.ID)
	return 0
}

// parseIDYesArgs extracts a positional task id and an anywhere -y/--yes flag,
// so both `abandon <id> -y` and `abandon -y <id>` read naturally.
func parseIDYesArgs(args []string) (id string, yes bool) {
	for _, a := range args {
		switch a {
		case "-y", "--yes":
			yes = true
		default:
			if id == "" {
				id = a
			}
		}
	}
	return id, yes
}

// gitAt returns a ship.Git bound to dir, backed by the real command runner.
func gitAt(dir string) ship.Git {
	return ship.Git{Runner: ship.ExecRunner{}, Dir: dir}
}

// short truncates a commit SHA for display.
func short(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}
