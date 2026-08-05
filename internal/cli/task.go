package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

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
	case "switch":
		return cmdTaskSwitch(a, rest)
	case "rm", "remove":
		return cmdTaskRemove(a, rest)
	default:
		fmt.Fprintf(os.Stderr, "karya task: unknown subcommand %q\n", sub)
		fmt.Fprintln(os.Stderr, "usage: karya task new|list|switch|rm")
		return 2
	}
}

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
	// The prompt is free-form positional text, so --agent may sit before or after
	// it. Go's flag package stops at the first positional, which would swallow a
	// trailing --agent into the prompt; parse order-independently instead.
	prompt, agentFlag := parseTaskNewArgs(args)
	if prompt == "" {
		fmt.Fprintln(os.Stderr, `usage: karya task new "<prompt>" [--agent <name>]`)
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

	id := task.NewID()
	wt, err := mgr.Add(top, id)
	if err != nil {
		return fail(err)
	}

	saved, err := store.Save(task.Task{
		ID:       id,
		Title:    task.TitleFromPrompt(prompt),
		Prompt:   prompt,
		Agent:    resolved,
		Status:   task.StatusWorking,
		Branch:   worktree.Branch(id),
		Worktree: wt,
		Repo:     top,
	})
	if err != nil {
		// Roll back the worktree so a failed save never leaks a checkout/branch.
		_ = mgr.Remove(top, id)
		return fail(err)
	}

	fmt.Printf("Created task %s — %s\n  branch:   %s\n  worktree: %s\n  agent:    %s\n",
		saved.ID, saved.Title, saved.Branch, saved.Worktree, saved.Agent)

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

// parseTaskNewArgs splits `task new` arguments into the free-form prompt and the
// optional --agent value, accepting the flag before or after the prompt and in
// `--agent x`, `--agent=x`, and single-dash forms.
func parseTaskNewArgs(args []string) (prompt, agentName string) {
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
		default:
			rest = append(rest, a)
		}
	}
	return strings.TrimSpace(strings.Join(rest, " ")), agentName
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSTATUS\tAGENT\tTITLE")
	for _, t := range tasks {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", t.ID, t.Status, t.Agent, t.Title)
	}
	_ = w.Flush()
	return 0
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
