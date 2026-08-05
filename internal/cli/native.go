package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/drjzlyan/karya/internal/native"
)

// cmdAgentNative runs karya's built-in Claude-API agent (ROADMAP Phase 13). With
// a prompt it runs one task; with no prompt it opens a REPL. Every file write or
// shell command the agent proposes pauses for the user's approval — the
// per-tool-call permission prompt that BYO-CLI agents cannot offer. Requires
// ANTHROPIC_API_KEY (override the model with KARYA_AGENT_MODEL).
func cmdAgentNative(a *app, args []string) int {
	client, ok := native.New()
	if !ok {
		fmt.Fprintln(os.Stderr, "karya agent native: set ANTHROPIC_API_KEY to use the built-in agent.")
		return 1
	}
	dir := sessionWorkdir(a)
	if dir == "" {
		dir, _ = os.Getwd()
	}

	// permit is the per-tool-call gate: write_file and run_command pause here.
	permit := func(action, detail string) bool {
		return confirm(os.Stdin, fmt.Sprintf("Allow the agent to %s: %s?", action, detail))
	}

	if prompt := strings.TrimSpace(strings.Join(args, " ")); prompt != "" {
		return runNative(client, dir, prompt, permit)
	}

	// REPL.
	fmt.Printf("karya native agent (%s) in %s — type a task, or 'exit' to quit.\n", client.Model, dir)
	sc := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\nagent> ")
		if !sc.Scan() {
			return 0
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if line == "exit" || line == "quit" {
			return 0
		}
		runNative(client, dir, line, permit)
	}
}

// runNative executes one native-agent task and reports failures.
func runNative(client *native.Client, dir, prompt string, permit native.Permit) int {
	if _, err := client.Run(context.Background(), dir, prompt, permit, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "karya agent native: %v\n", err)
		return 1
	}
	return 0
}
