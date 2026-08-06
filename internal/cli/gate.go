package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/drjzlyan/karya/internal/gate"
	"github.com/drjzlyan/karya/internal/git"
	"github.com/drjzlyan/karya/internal/review"
	"github.com/drjzlyan/karya/internal/task"
)

// cmdGate implements `karya gate list|approve|reject|delegate`: the human side
// of the mandatory gates (DESIGN.md §2, §6). Every crossing is recorded in
// STATE.json with its actor and (for rejections) feedback.
func cmdGate(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: karya gate list|approve|reject|delegate <id> [...]")
		return 2
	}
	a, err := newApp()
	if err != nil {
		return fail(err)
	}
	_, store, _, err := taskContext(a)
	if err != nil {
		return fail(err)
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "list":
		return gateList(store)
	case "approve":
		return gateCross(store, rest, false)
	case "reject":
		return gateCross(store, rest, true)
	case "delegate":
		return gateDelegate(store, rest)
	default:
		fmt.Fprintf(os.Stderr, "unknown gate subcommand %q\n", sub)
		return 2
	}
}

// gateList prints the gate inbox: tasks awaiting a human crossing.
func gateList(store *task.Store) int {
	tasks, err := store.List()
	if err != nil {
		return fail(err)
	}
	pending := gate.PendingTasks(tasks)
	if len(pending) == 0 {
		fmt.Println("No tasks awaiting a gate.")
		return 0
	}
	fmt.Println("Tasks awaiting a gate:")
	for _, t := range pending {
		p, _ := gate.For(t.State)
		fmt.Printf("  %-28s %-12s gate:%s\n", t.ID, t.State, p.Gate)
	}
	return 0
}

// gateCross approves or rejects the pending gate of a task as the human.
func gateCross(store *task.Store, args []string, reject bool) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: karya gate approve|reject <id> [feedback…]")
		return 2
	}
	id := args[0]
	feedback := strings.TrimSpace(strings.Join(args[1:], " "))
	return cross(store, id, "human", reject, feedback)
}

// gateDelegate approves a gate as an agent (a recorded delegation).
func gateDelegate(store *task.Store, args []string) int {
	fs := flag.NewFlagSet("gate delegate", flag.ContinueOnError)
	to := fs.String("to", "", "agent to delegate the approval to")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 || *to == "" {
		fmt.Fprintln(os.Stderr, "usage: karya gate delegate <id> --to <agent>")
		return 2
	}
	return cross(store, fs.Arg(0), "agent:"+*to, false, "")
}

// cross performs the actual state transition and records it.
func cross(store *task.Store, id, actor string, reject bool, feedback string) int {
	t, err := store.Get(id)
	if err != nil {
		return fail(err)
	}
	p, ok := gate.For(t.State)
	if !ok {
		fmt.Fprintf(os.Stderr, "task %s is not awaiting a gate (state: %s)\n", id, t.State)
		return 2
	}
	target := p.Approve
	if reject {
		target = p.Reject
		if feedback == "" {
			fmt.Fprintln(os.Stderr, "reject requires feedback: karya gate reject <id> <feedback…>")
			return 2
		}
	}
	if err := t.Transition(target, actor, feedback); err != nil {
		return fail(err)
	}
	if err := store.Save(t); err != nil {
		return fail(err)
	}
	verb := "approved"
	if reject {
		verb = "rejected"
	}
	fmt.Printf("%s gate:%s %s → %s (by %s)\n", verb, p.Gate, id, t.State, actor)
	return 0
}

// cmdReview implements `karya review <id>`: it assembles and prints a task's
// reviewable artifacts (spec, plan, diff, evidence) and its pending gate.
func cmdReview(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: karya review <id>")
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
	r, err := review.Assemble(store, git.New(top, nil), args[0])
	if err != nil {
		return fail(err)
	}
	printReview(r)
	return 0
}

func printReview(r *review.Review) {
	fmt.Printf("Task %s  [%s]\n", r.Task.ID, r.Task.State)
	if r.HasGate {
		fmt.Printf("Pending gate: %s  (approve → %s, reject → %s)\n", r.Pending.Gate, r.Pending.Approve, r.Pending.Reject)
	}
	if r.Spec != nil && r.Spec.Objective != "" {
		fmt.Printf("\n## Objective\n%s\n", strings.TrimSpace(r.Spec.Objective))
	}
	if r.Plan != "" {
		fmt.Printf("\n## Plan\n%s\n", strings.TrimSpace(r.Plan))
	}
	if r.Diff != "" {
		fmt.Printf("\n## Diff\n%s\n", strings.TrimSpace(r.Diff))
	}
	for i, e := range r.Evidence {
		fmt.Printf("\n## Verification %d\n%s\n", i+1, strings.TrimSpace(e))
	}
	if r.HasGate {
		fmt.Printf("\nApprove: karya gate approve %s   Reject: karya gate reject %s <feedback>\n", r.Task.ID, r.Task.ID)
	}
}
