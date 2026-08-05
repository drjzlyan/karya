package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/drjzlyan/karya/internal/agent"
	"github.com/drjzlyan/karya/internal/session"
	"github.com/drjzlyan/karya/internal/ship"
	"github.com/drjzlyan/karya/internal/task"
	"github.com/drjzlyan/karya/internal/worktree"
)

// cmdTask dispatches `karya task <subcommand>`. Tasks are karya's unit of agent
// work: each runs in its own isolated git worktree (branch karya/<id>) under the
// karya prefix, so an agent's changes are contained and reviewable before they
// touch the user's real branch (ROADMAP Phases 10–11).
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
	case "dashboard":
		return cmdTaskDashboard(a)
	case "switch":
		return cmdTaskSwitch(a, rest)
	case "rm", "remove":
		return cmdTaskRemove(a, rest)
	case "plan":
		return cmdTaskPlan(a, rest)
	case "approve-plan":
		return cmdTaskApprovePlan(a, rest)
	case "review":
		return cmdTaskReview(a, rest)
	case "merge":
		return cmdTaskMerge(a, rest)
	case "reject":
		return cmdTaskReject(a, rest)
	case "checkpoint":
		return cmdTaskCheckpoint(a, rest)
	case "rewind":
		return cmdTaskRewind(a, rest)
	case "allow":
		return cmdTaskAllow(a, rest)
	default:
		fmt.Fprintf(os.Stderr, "karya task: unknown subcommand %q\n", sub)
		fmt.Fprintln(os.Stderr,
			"usage: karya task new|list|switch|rm|plan|approve-plan|review|merge|reject|checkpoint|rewind|allow")
		return 2
	}
}

// gitAt returns a ship.Git bound to dir, backed by the real command runner. It is
// the single home for the git plumbing the gates share (review diff, merge,
// checkpoint commit, rewind reset).
func gitAt(dir string) ship.Git {
	return ship.Git{Runner: ship.ExecRunner{}, Dir: dir}
}

// gateAction asks for permission before karya takes a side-effecting action it
// initiates (merge, push, rewind). It is satisfied by an explicit -y, by a
// per-project allowlist entry (`karya task allow <action>`), or by an
// interactive yes. NOTE: this gates only karya's own actions — it cannot
// intercept a BYO-CLI agent's internal tool calls; that arrives with the native
// engine (ROADMAP Phase 13).
func gateAction(a *app, repo, actionKey, phrase string, yes bool) bool {
	if yes || a.prefs.Get(allowKey(repo, actionKey)) == "1" {
		return true
	}
	return confirm(os.Stdin, "Allow karya to "+phrase+"?")
}

// allowKey is the per-project prefs key authorizing a karya-initiated action.
func allowKey(repo, action string) string { return "allow." + repo + "." + action }

// taskContext resolves the git repository the caller is in and returns the
// worktree Manager and per-project task Store bound to it. Every task command
// operates on the repository containing the cwd (or the current session's
// workdir), so tasks stay grouped by project.
func taskContext(a *app) (worktree.Manager, *task.Store, string, error) {
	dir := sessionWorkdir(a)
	if dir == "" {
		dir, _ = os.Getwd()
	}
	runner := ship.ExecRunner{}
	top, err := runner.Output(dir, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return worktree.Manager{}, nil, "", fmt.Errorf("karya task must run inside a git repository: %w", err)
	}
	top = strings.TrimSpace(top)
	mgr := worktree.Manager{Runner: runner, Root: a.paths.WorktreesDir()}
	store := task.NewStore(filepath.Join(a.paths.TasksDir(), worktree.ProjectSlug(top)+".json"))
	return mgr, store, top, nil
}

// cmdTaskNew creates a task: an isolated worktree on branch karya/<id> plus a
// stored record, then opens (when inside a karya session) a session rooted at
// that worktree with the resolved agent. The prompt is kept with the task; the
// agent works inside the worktree, and the human reviews before merge (Phase 11).
func cmdTaskNew(a *app, args []string) int {
	// The prompt is free-form positional text, so the flags may sit before or
	// after it. Go's flag package stops at the first positional, which would
	// swallow a trailing --agent into the prompt; parse order-independently.
	prompt, agentFlag, plan := parseTaskNewArgs(args)
	if prompt == "" {
		fmt.Fprintln(os.Stderr, `usage: karya task new "<prompt>" [--agent <name>] [--plan]`)
		return 2
	}

	mgr, store, top, err := taskContext(a)
	if err != nil {
		return fail(err)
	}

	// Agent resolution mirrors `karya dev`: flag → saved per-project preference →
	// single detected → interactive picker.
	detected := agent.Detect()
	explicit := agentFlag
	if explicit == "" {
		explicit = a.prefs.Get("agent." + top)
	}
	resolved := agent.Resolve(explicit, detected)

	// Record the commit the task branch forks from so a later review can diff the
	// whole task against exactly where it started.
	base, _ := gitAt(top).RevParse("HEAD")

	id := task.NewID()
	wt, err := mgr.Add(top, id)
	if err != nil {
		return fail(err)
	}

	t := task.Task{
		ID:         id,
		Title:      task.TitleFromPrompt(prompt),
		Prompt:     prompt,
		Agent:      resolved,
		Status:     task.StatusWorking,
		Branch:     worktree.Branch(id),
		Worktree:   wt,
		Repo:       top,
		BaseCommit: base,
	}

	// Plan gate: when requested, draft a plan with the agent's headless mode and
	// hold the task at awaiting-plan until the human approves it.
	if plan {
		t = draftPlan(t, resolved)
	}

	saved, err := store.Save(t)
	if err != nil {
		// Roll back the worktree so a failed save never leaks a checkout/branch.
		_ = mgr.Remove(top, id)
		return fail(err)
	}

	fmt.Printf("Created task %s — %s\n  branch:   %s\n  worktree: %s\n  agent:    %s\n  status:   %s\n",
		saved.ID, saved.Title, saved.Branch, saved.Worktree, saved.Agent, saved.Status)

	if saved.Status == task.StatusAwaitingPlan {
		if saved.Plan != "" {
			fmt.Printf("\nProposed plan:\n\n%s\n", saved.Plan)
		}
		fmt.Printf("\nApprove with: karya task approve-plan %s\n", saved.ID)
		return 0
	}

	// Only when we are inside a karya session do we open the task session — and
	// only then is it worth provisioning the runtime. A standalone `task new`
	// just records the task; `karya task switch` provisions and attaches later.
	if os.Getenv("TMUX") != "" {
		if err := ensureRuntime(a); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}
		if openTaskSession(a, saved) {
			return 0
		}
	}
	fmt.Printf("Switch to it with: karya task switch %s\n", saved.ID)
	return 0
}

// draftPlan asks the agent to author an implementation plan via its headless
// mode and returns the task moved to awaiting-plan. When the agent has no
// headless mode (or is none), it still parks the task at awaiting-plan with an
// empty plan so the human can write or approve one — the gate holds either way.
func draftPlan(t task.Task, agentName string) task.Task {
	t.Status = task.StatusAwaitingPlan
	if !agent.SupportsHeadless(agentName) {
		fmt.Fprintf(os.Stderr,
			"note: %s has no headless mode; write the plan yourself or approve to proceed.\n", agentName)
		return t
	}
	fmt.Printf("Asking %s to draft a plan…\n", agentName)
	out, err := agent.NewCLIRunner(agentName).Headless(context.Background(), t.Worktree, planPrompt(t.Prompt))
	if err != nil {
		fmt.Fprintf(os.Stderr, "note: could not draft a plan (%v); approve to proceed.\n", err)
		return t
	}
	t.Plan = strings.TrimSpace(out)
	return t
}

// planPrompt is the instruction handed to the agent to produce a review-ready
// implementation plan and nothing else.
func planPrompt(prompt string) string {
	return "Produce a concise, numbered implementation plan for the task below. " +
		"Output ONLY the plan — no preamble, no code, no commentary.\n\nTask: " + prompt
}

// parseTaskNewArgs splits `task new` arguments into the free-form prompt, the
// optional --agent value, and the --plan flag, accepting the flags before or
// after the prompt and in `--agent x`, `--agent=x`, and single-dash forms.
func parseTaskNewArgs(args []string) (prompt, agentName string, plan bool) {
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--agent" || a == "-agent":
			if i+1 < len(args) {
				agentName = args[i+1]
				i++
			}
		case strings.HasPrefix(a, "--agent="):
			agentName = strings.TrimPrefix(a, "--agent=")
		case strings.HasPrefix(a, "-agent="):
			agentName = strings.TrimPrefix(a, "-agent=")
		case a == "--plan" || a == "-plan":
			plan = true
		default:
			rest = append(rest, a)
		}
	}
	return strings.TrimSpace(strings.Join(rest, " ")), agentName, plan
}

// cmdTaskList prints the project's tasks, newest last, as an aligned table.
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
		fmt.Println(`No tasks yet. Create one with: karya task new "<prompt>"`)
		return 0
	}
	fmt.Print(renderTasks(tasks, false))
	return 0
}

// renderTasks returns an aligned table of tasks. When numbered, a leading 1-based
// index column lets the dashboard pick a task by number.
func renderTasks(tasks []task.Task, numbered bool) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	if numbered {
		fmt.Fprintln(w, "#\tID\tSTATUS\tAGENT\tTITLE")
		for i, t := range tasks {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", i+1, t.ID, t.Status, t.Agent, t.Title)
		}
	} else {
		fmt.Fprintln(w, "ID\tSTATUS\tAGENT\tTITLE")
		for _, t := range tasks {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", t.ID, t.Status, t.Agent, t.Title)
		}
	}
	_ = w.Flush()
	return b.String()
}

// cmdTaskDashboard is the fleet view (bound to Ctrl-a T via a tmux popup): it
// lists every task with its live status and, interactively, switches to the one
// the user picks. Running many tasks at once is already supported — each is its
// own worktree and session — so this is the single place to see and navigate the
// fleet.
func cmdTaskDashboard(a *app) int {
	_, store, _, err := taskContext(a)
	if err != nil {
		return fail(err)
	}
	tasks, err := store.List()
	if err != nil {
		return fail(err)
	}
	fmt.Println("karya · task fleet")
	fmt.Println()
	if len(tasks) == 0 {
		fmt.Println(`No tasks yet. Create one with: karya task new "<prompt>"`)
		return 0
	}
	fmt.Print(renderTasks(tasks, true))
	if !isInteractive() {
		return 0
	}
	fmt.Print("\nEnter a task # (or id) to switch, or q to quit: ")
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	choice := strings.TrimSpace(line)
	if choice == "" || choice == "q" {
		return 0
	}
	idx := dashboardChoice(choice, tasks)
	if idx < 0 {
		fmt.Println("No such task.")
		return 0
	}
	t := tasks[idx]
	if openTaskSession(a, t) {
		return 0
	}
	if err := ensureRuntime(a); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}
	if err := session.Dev(a.tmux, taskSessionOptions(t)); err != nil {
		return fail(err)
	}
	return 0
}

// dashboardChoice resolves a user's dashboard input to a task index: a 1-based
// number, or an id (prefix) match. It returns -1 when nothing matches.
func dashboardChoice(choice string, tasks []task.Task) int {
	if n, err := strconv.Atoi(choice); err == nil {
		if n >= 1 && n <= len(tasks) {
			return n - 1
		}
		return -1
	}
	for i, t := range tasks {
		if strings.HasPrefix(t.ID, choice) {
			return i
		}
	}
	return -1
}

// cmdTaskSwitch attaches to (or, inside tmux, switches the client to) the task's
// session, rooted at its isolated worktree.
func cmdTaskSwitch(a *app, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: karya task switch <id>")
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
	if openTaskSession(a, t) {
		return 0
	}
	// Not inside a karya session: hand over the terminal to the task session.
	if err := ensureRuntime(a); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}
	if err := session.Dev(a.tmux, taskSessionOptions(t)); err != nil {
		return fail(err)
	}
	return 0
}

// cmdTaskRemove tears a task down: its worktree checkout, its karya/<id> branch,
// and its stored record. It confirms first because this discards the agent's work
// (pass -y to skip).
func cmdTaskRemove(a *app, args []string) int {
	// Order-independent so `task rm <id> -y` works (Go's flag would stop at <id>).
	id, yes := parseTaskRemoveArgs(args)
	if id == "" {
		fmt.Fprintln(os.Stderr, "usage: karya task rm <id> [-y]")
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
	if !yes && !confirm(os.Stdin, fmt.Sprintf("Remove task %s (%s) and its worktree/branch?", t.ID, t.Title)) {
		fmt.Println("Aborted.")
		return 0
	}
	// Kill any live session for the task before pulling its worktree out.
	name := taskSessionName(t)
	if a.tmux.HasSession(name) {
		_ = a.tmux.Run("kill-session", "-t", name)
	}
	if err := mgr.Remove(top, id); err != nil {
		return fail(err)
	}
	if err := store.Delete(id); err != nil {
		return fail(err)
	}
	fmt.Printf("Removed task %s.\n", id)
	return 0
}

// parseTaskRemoveArgs pulls the task id and the -y/--yes flag from `task rm`
// arguments regardless of their order.
func parseTaskRemoveArgs(args []string) (id string, yes bool) {
	for _, a := range args {
		switch a {
		case "-y", "--yes", "-yes":
			yes = true
		default:
			if id == "" && !strings.HasPrefix(a, "-") {
				id = a
			}
		}
	}
	return id, yes
}

// currentTaskID returns the task whose session the caller is inside — task
// sessions are named "task-<id>" — or "" when not in one. It lets the gate
// commands (review/merge/checkpoint/…) default their <id> to the current task,
// so the editor keymaps and an in-session human can omit it.
func currentTaskID(a *app) string {
	if os.Getenv("TMUX") == "" {
		return ""
	}
	name, err := a.tmux.Output("display-message", "-p", "#{session_name}")
	if err != nil {
		return ""
	}
	if name = strings.TrimSpace(name); strings.HasPrefix(name, "task-") {
		return strings.TrimPrefix(name, "task-")
	}
	return ""
}

// resolveTaskID returns explicit when non-empty, else the current task session's
// id.
func resolveTaskID(a *app, explicit string) string {
	if explicit != "" {
		return explicit
	}
	return currentTaskID(a)
}

// taskByID loads a task by id within the caller's project.
func taskByID(a *app, id string) (worktree.Manager, *task.Store, string, task.Task, error) {
	mgr, store, top, err := taskContext(a)
	if err != nil {
		return mgr, store, top, task.Task{}, err
	}
	t, err := store.Get(id)
	return mgr, store, top, t, err
}

// cmdTaskPlan prints a task's drafted plan (the plan-approval gate's artifact).
func cmdTaskPlan(a *app, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: karya task plan <id>")
		return 2
	}
	_, _, _, t, err := taskByID(a, args[0])
	if err != nil {
		return fail(err)
	}
	if strings.TrimSpace(t.Plan) == "" {
		fmt.Println("No plan recorded for this task.")
		return 0
	}
	fmt.Println(t.Plan)
	return 0
}

// cmdTaskApprovePlan clears the plan gate: awaiting-plan → working, then opens
// the task session so the agent can start on the approved plan.
func cmdTaskApprovePlan(a *app, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: karya task approve-plan <id>")
		return 2
	}
	_, store, _, t, err := taskByID(a, args[0])
	if err != nil {
		return fail(err)
	}
	if t.Status != task.StatusAwaitingPlan {
		return fail(fmt.Errorf("task %s is not awaiting plan approval (status: %s)", t.ID, t.Status))
	}
	t.Status = task.StatusWorking
	saved, err := store.Save(t)
	if err != nil {
		return fail(err)
	}
	fmt.Printf("Plan approved; task %s is now working.\n", saved.ID)
	if os.Getenv("TMUX") != "" {
		if err := ensureRuntime(a); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}
		if openTaskSession(a, saved) {
			return 0
		}
	}
	fmt.Printf("Switch to it with: karya task switch %s\n", saved.ID)
	return 0
}

// cmdTaskReview is the diff-review gate: it shows the whole of a task's changes
// (against its base commit) so the human can decide to merge or reject. Because
// the agent worked in an isolated worktree, nothing has touched the user's branch
// yet — this is the pre-apply review.
func cmdTaskReview(a *app, args []string) int {
	id := resolveTaskID(a, firstPositional(args))
	if id == "" {
		fmt.Fprintln(os.Stderr, "usage: karya task review <id>")
		return 2
	}
	_, store, _, t, err := taskByID(a, id)
	if err != nil {
		return fail(err)
	}
	diff, err := taskDiff(t)
	if err != nil {
		return fail(err)
	}
	if strings.TrimSpace(diff) == "" {
		fmt.Println("No changes in this task's worktree yet.")
		return 0
	}
	if t.Status == task.StatusWorking {
		t.Status = task.StatusAwaitingReview
		if _, err := store.Save(t); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}
	}
	fmt.Print(diff)
	if !strings.HasSuffix(diff, "\n") {
		fmt.Println()
	}
	fmt.Printf("\nApply with: karya task merge %s   ·   discard with: karya task reject %s\n", t.ID, t.ID)
	return 0
}

// taskDiff stages the worktree (so new files are included) and returns the diff
// of the whole task against its base commit.
func taskDiff(t task.Task) (string, error) {
	g := gitAt(t.Worktree)
	if err := g.StageAll(); err != nil {
		return "", err
	}
	if t.BaseCommit == "" {
		// A task created before base tracking: fall back to the staged diff.
		return g.StagedDiff()
	}
	return g.DiffCachedAgainst(t.BaseCommit)
}

// cmdTaskMerge applies a reviewed task: it commits any in-progress worktree edits
// onto the task branch, then merges that branch into the project's current
// branch. The merge (and optional --push) are permission-gated.
func cmdTaskMerge(a *app, args []string) int {
	id, yes := parseTaskRemoveArgs(args)
	id = resolveTaskID(a, id)
	if id == "" {
		fmt.Fprintln(os.Stderr, "usage: karya task merge <id> [--push] [-y]")
		return 2
	}
	_, store, top, t, err := taskByID(a, id)
	if err != nil {
		return fail(err)
	}
	if !gateAction(a, top, "merge", "merge task "+t.ID, yes) {
		fmt.Println("Aborted.")
		return 0
	}
	if _, err := gitAt(t.Worktree).CommitAll(fmt.Sprintf("task %s: %s", t.ID, t.Title), false); err != nil {
		return fail(err)
	}
	if err := gitAt(top).Merge(t.Branch, true); err != nil {
		return fail(fmt.Errorf("merge failed (resolve conflicts in %s, or reject the task): %w", top, err))
	}
	t.Status = task.StatusMerged
	if _, err := store.Save(t); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}
	fmt.Printf("Merged %s into %s.\n", t.Branch, top)
	if hasFlag(args, "--push") {
		if !gateAction(a, top, "push", "push", yes) {
			fmt.Println("Skipped push.")
		} else if err := gitAt(top).Push(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: push failed: %v\n", err)
		}
	}
	fmt.Printf("Remove the task's worktree with: karya task rm %s\n", t.ID)
	return 0
}

// cmdTaskReject marks a task rejected without applying it. The worktree is kept
// so the human can inspect it; `karya task rm` clears it.
func cmdTaskReject(a *app, args []string) int {
	id, _ := parseTaskRemoveArgs(args)
	id = resolveTaskID(a, id)
	if id == "" {
		fmt.Fprintln(os.Stderr, "usage: karya task reject <id>")
		return 2
	}
	_, store, _, t, err := taskByID(a, id)
	if err != nil {
		return fail(err)
	}
	t.Status = task.StatusRejected
	if _, err := store.Save(t); err != nil {
		return fail(err)
	}
	fmt.Printf("Rejected task %s. Remove its worktree with: karya task rm %s\n", t.ID, t.ID)
	return 0
}

// cmdTaskCheckpoint records a restorable snapshot of a task's worktree: it
// commits the current state on the task branch and remembers the commit so
// `karya task rewind` can return to it.
func cmdTaskCheckpoint(a *app, args []string) int {
	// With no args, checkpoint the current task session with a default label;
	// otherwise the first arg is the id and the rest is the label.
	var id, label string
	if len(args) > 0 {
		id, label = args[0], strings.TrimSpace(strings.Join(args[1:], " "))
	} else {
		id = currentTaskID(a)
	}
	if id == "" {
		fmt.Fprintln(os.Stderr, "usage: karya task checkpoint <id> [label]")
		return 2
	}
	if label == "" {
		label = "checkpoint"
	}
	_, store, _, t, err := taskByID(a, id)
	if err != nil {
		return fail(err)
	}
	g := gitAt(t.Worktree)
	if _, err := g.CommitAll("checkpoint: "+label, false); err != nil {
		return fail(err)
	}
	sha, err := g.RevParse("HEAD")
	if err != nil {
		return fail(err)
	}
	t.Checkpoints = append(t.Checkpoints, task.Checkpoint{SHA: sha, Label: label, Created: time.Now().UTC()})
	if _, err := store.Save(t); err != nil {
		return fail(err)
	}
	fmt.Printf("Checkpoint %d saved (%s) at %s.\n", len(t.Checkpoints), label, short(sha))
	return 0
}

// cmdTaskRewind restores a task's worktree to a checkpoint, discarding changes
// made after it. The target is a 1-based checkpoint index or a SHA prefix;
// omitted, it rewinds to the most recent checkpoint. It is permission-gated
// because it is destructive to the worktree.
func cmdTaskRewind(a *app, args []string) int {
	id, target, yes := parseTaskRewindArgs(args)
	id = resolveTaskID(a, id)
	if id == "" {
		fmt.Fprintln(os.Stderr, "usage: karya task rewind <id> [index|sha] [-y]")
		return 2
	}
	_, _, top, t, err := taskByID(a, id)
	if err != nil {
		return fail(err)
	}
	if len(t.Checkpoints) == 0 {
		return fail(fmt.Errorf("task %s has no checkpoints; create one with: karya task checkpoint %s", t.ID, t.ID))
	}
	cp, err := resolveCheckpoint(t, target)
	if err != nil {
		return fail(err)
	}
	if !gateAction(a, top, "rewind", "rewind (discard changes after the checkpoint)", yes) {
		fmt.Println("Aborted.")
		return 0
	}
	if err := gitAt(t.Worktree).ResetHard(cp.SHA); err != nil {
		return fail(err)
	}
	fmt.Printf("Rewound task %s to checkpoint %q (%s).\n", t.ID, cp.Label, short(cp.SHA))
	return 0
}

// cmdTaskAllow pre-authorizes a karya-initiated action for this project so its
// permission prompt is skipped going forward.
func cmdTaskAllow(a *app, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: karya task allow <merge|push|rewind>")
		return 2
	}
	action := args[0]
	switch action {
	case "merge", "push", "rewind":
	default:
		return fail(fmt.Errorf("unknown action %q (allow: merge, push, rewind)", action))
	}
	_, _, top, err := taskContext(a)
	if err != nil {
		return fail(err)
	}
	if err := a.prefs.Set(allowKey(top, action), "1"); err != nil {
		return fail(err)
	}
	fmt.Printf("karya will no longer prompt before %q in this project.\n", action)
	return 0
}

// resolveCheckpoint selects a checkpoint by 1-based index, by SHA prefix, or —
// when target is empty — the most recent one.
func resolveCheckpoint(t task.Task, target string) (task.Checkpoint, error) {
	cps := t.Checkpoints
	if target == "" {
		return cps[len(cps)-1], nil
	}
	if n, err := strconv.Atoi(target); err == nil {
		if n < 1 || n > len(cps) {
			return task.Checkpoint{}, fmt.Errorf("checkpoint %d out of range (1..%d)", n, len(cps))
		}
		return cps[n-1], nil
	}
	for _, cp := range cps {
		if strings.HasPrefix(cp.SHA, target) {
			return cp, nil
		}
	}
	return task.Checkpoint{}, fmt.Errorf("no checkpoint matching %q", target)
}

// parseTaskRewindArgs pulls the id, an optional target (index or sha), and -y
// from `task rewind` arguments regardless of order.
func parseTaskRewindArgs(args []string) (id, target string, yes bool) {
	var pos []string
	for _, a := range args {
		switch a {
		case "-y", "--yes", "-yes":
			yes = true
		default:
			if !strings.HasPrefix(a, "-") {
				pos = append(pos, a)
			}
		}
	}
	if len(pos) > 0 {
		id = pos[0]
	}
	if len(pos) > 1 {
		target = pos[1]
	}
	return id, target, yes
}

// firstPositional returns the first non-flag argument, or "".
func firstPositional(args []string) string {
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			return a
		}
	}
	return ""
}

// hasFlag reports whether flag appears in args.
func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

// short returns the first 8 characters of a SHA (or the whole string if shorter).
func short(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

// taskSessionName is the tmux session name for a task.
func taskSessionName(t task.Task) string { return sanitizeSession("task-" + t.ID) }

// taskSessionOptions builds the session layout options for a task: rooted at its
// isolated worktree, running the task's agent.
func taskSessionOptions(t task.Task) session.Options {
	return session.Options{
		Name:     taskSessionName(t),
		Workdir:  t.Worktree,
		Agent:    t.Agent,
		Detected: agent.Detect(),
	}
}

// openTaskSession builds (if needed) and switches to the task's session when the
// caller is inside a karya tmux session. It returns false when not in one, so the
// caller can fall back to attaching or printing instructions. It mirrors
// openInSession but roots the session at the task's isolated worktree.
func openTaskSession(a *app, t task.Task) bool {
	if os.Getenv("TMUX") == "" {
		return false
	}
	name := taskSessionName(t)
	if !a.tmux.HasSession(name) {
		if err := session.Build(a.tmux, taskSessionOptions(t)); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not open task session: %v\n", err)
			return false
		}
	}
	_ = a.tmux.Run("switch-client", "-t", name+":dev")
	fmt.Printf("Switched to task session %q\n", name)
	return true
}
