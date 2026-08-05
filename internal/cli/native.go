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
	// One reader for both the REPL prompt and the per-tool-call approvals. They
	// share stdin, so they must share a single buffered reader — otherwise the
	// REPL scanner and the approval reader keep separate buffers over the same fd
	// and steal each other's input.
	reader := bufio.NewReader(os.Stdin)
	permit := func(action, detail string) bool {
		return askYesNo(reader, fmt.Sprintf("Allow the agent to %s: %s?", action, detail))
	}

	if prompt := strings.TrimSpace(strings.Join(args, " ")); prompt != "" {
		return runNative(client, dir, prompt, permit)
	}

	// REPL.
	fmt.Printf("karya native agent (%s) in %s — type a task, or 'exit' to quit.\n", client.Model, dir)
	for {
		fmt.Print("\nagent> ")
		raw, err := reader.ReadString('\n')
		line := strings.TrimSpace(raw)
		if line == "exit" || line == "quit" {
			return 0
		}
		if line != "" {
			runNative(client, dir, line, permit)
		}
		if err != nil { // EOF (Ctrl-D) after handling any trailing input
			return 0
		}
	}
}

// askYesNo prompts on the shared reader and returns true only for an explicit
// yes. It mirrors confirm but reuses the caller's reader so it never competes
// with another reader over the same stdin.
func askYesNo(r *bufio.Reader, prompt string) bool {
	fmt.Printf("%s [y/N] ", prompt)
	line, err := r.ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	default:
		return false
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
