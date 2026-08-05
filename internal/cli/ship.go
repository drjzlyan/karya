package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/drjzlyan/karya/internal/agent"
	"github.com/drjzlyan/karya/internal/ship"
)

// cmdShip runs karya's agent-driven git flow: stage the work tree, have the
// active coding agent author a Conventional-Commit message, then commit and
// (optionally) push / open a PR. The agent only writes the message string;
// every repository mutation is plain git so the flow stays predictable.
func cmdShip(args []string) int {
	fs := flag.NewFlagSet("ship", flag.ContinueOnError)
	push := fs.Bool("push", false, "push the branch after committing")
	pr := fs.Bool("pr", false, "open a pull request (implies --push; requires gh)")
	noVerify := fs.Bool("no-verify", false, "skip git commit hooks")
	yes := fs.Bool("y", false, "commit without the confirmation prompt")
	fs.BoolVar(yes, "yes", false, "commit without the confirmation prompt")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	a, err := newApp()
	if err != nil {
		return fail(err)
	}

	dir := sessionWorkdir(a)
	if dir == "" {
		dir, _ = os.Getwd()
	}

	g := ship.Git{Runner: ship.ExecRunner{}, Dir: dir}
	if !g.InsideRepo() {
		return fail(fmt.Errorf("%s is not a git repository", dir))
	}
	if err := g.StageAll(); err != nil {
		return fail(err)
	}
	staged, err := g.HasStaged()
	if err != nil {
		return fail(err)
	}
	if !staged {
		fmt.Println("Nothing to commit — working tree clean.")
		return 0
	}
	diff, err := g.StagedDiff()
	if err != nil {
		return fail(err)
	}

	// Author the message with the active agent's headless mode. When that is
	// unavailable, hand the whole task to the agent in its pane (conversational
	// fallback) and let the human drive it from there.
	current := shipAgent(a)
	message := shipMessage(current, dir, diff)
	if message == "" {
		return shipFallback(a, current)
	}

	fmt.Printf("\nProposed commit message (%s):\n\n%s\n\n", current, indentBlock(message))
	if !*yes && isInteractive() && !confirm(os.Stdin, "Commit with this message?") {
		fmt.Println("Aborted.")
		return 1
	}
	if err := g.Commit(message, *noVerify); err != nil {
		return fail(err)
	}
	fmt.Println("Committed.")

	if *push || *pr {
		if err := g.Push(); err != nil {
			return fail(err)
		}
		fmt.Println("Pushed.")
	}
	if *pr {
		if err := g.CreatePR(ship.Subject(message), message); err != nil {
			return fail(err)
		}
		fmt.Println("Pull request opened.")
	}
	return 0
}

// shipAgent resolves the agent that should author the message: the session's
// current agent if we are inside one, otherwise the first detected agent.
func shipAgent(a *app) string {
	if m, ok := agentManager(a); ok {
		if cur := m.Current(); cur != "" && cur != agent.None {
			return cur
		}
	}
	if detected := agent.Detect(); len(detected) > 0 {
		return detected[0]
	}
	return ""
}

// shipMessage asks the agent to author a commit message from the staged diff via
// its headless mode, returning "" when the agent has no headless mode, is not on
// PATH, errors, or replies emptily — every one of which triggers the fallback.
// It drives the agent through its Runner so the flow is engine-agnostic.
func shipMessage(name, dir, diff string) string {
	if name == "" || !agent.SupportsHeadless(name) {
		return ""
	}
	fmt.Printf("Asking %s to write a commit message…\n", name)
	out, err := agent.NewCLIRunner(name).Headless(context.Background(), dir, ship.BuildPrompt(diff))
	if err != nil {
		// Explain why we are falling back to the conversational pane instead of
		// silently producing no message.
		fmt.Fprintf(os.Stderr, "karya ship: %s could not draft a message: %v\n", name, err)
		return ""
	}
	return ship.SanitizeMessage(out)
}

// shipFallback delegates the whole commit to the agent pane when no headless
// message could be produced. The staged changes are already prepared, so we just
// ask the agent (conversationally) to review and commit them.
func shipFallback(a *app, name string) int {
	m, ok := agentManager(a)
	if !ok || name == "" {
		fmt.Fprintln(os.Stderr, "karya ship: no agent available to author a commit message.")
		fmt.Fprintln(os.Stderr, "Changes are staged — commit manually, or start an agent and retry.")
		return 1
	}
	prompt := "The changes for this task are staged. Please review the staged diff, " +
		"write a Conventional Commits message, and create the commit."
	if err := m.Send(prompt, ""); err != nil {
		return fail(err)
	}
	fmt.Printf("Handed the commit to %s in the agent pane (changes are staged).\n", name)
	return 0
}

// isInteractive reports whether stdin is a terminal, so ship only prompts when a
// human can answer (the tmux popup binding is interactive; run-shell is not).
func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// indentBlock indents every line of s by two spaces for display.
func indentBlock(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = "  " + l
	}
	return strings.Join(lines, "\n")
}
