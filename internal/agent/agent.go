// Package agent detects and (from Phase 2) manages the coding agents that make
// karya "AI-first". Phase 1 provides detection and selection so the session can
// launch an agent in its pane; switch/next/prev/reset arrive in Phase 2.
package agent

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Known lists supported coding-agent CLIs in preference order. The first one
// found on PATH wins when no explicit choice is made.
var Known = []string{"crush", "claude", "codex", "gemini", "aider", "copilot"}

// None is the sentinel for "no agent, just a shell".
const None = "none"

// Available reports whether a command is on PATH.
func Available(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// Detect returns the known agents currently installed, in preference order.
func Detect() []string {
	var found []string
	for _, a := range Known {
		if Available(a) {
			found = append(found, a)
		}
	}
	return found
}

// Resolve picks the agent to launch given an explicit flag and the detected
// list. Order: explicit flag → single detected → interactive pick (tty) → none.
func Resolve(explicit string, detected []string) string {
	if explicit != "" {
		return explicit
	}
	switch len(detected) {
	case 0:
		return None
	case 1:
		return detected[0]
	default:
		return pick(detected)
	}
}

// pick prompts the user to choose among multiple agents. On a non-interactive
// stdin it falls back to the first (highest-preference) agent.
func pick(agents []string) string {
	if !stdinIsTerminal() {
		return agents[0]
	}
	fmt.Fprint(os.Stderr, "\n  Multiple coding agents detected:\n\n")
	for i, a := range agents {
		fmt.Fprintf(os.Stderr, "  %d. %s\n", i+1, a)
	}
	fmt.Fprintln(os.Stderr, "  0. none (just a shell)")
	fmt.Fprintln(os.Stderr, "\n  Enter a number or name, or Enter for #1.")
	fmt.Fprint(os.Stderr, "  > ")

	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return agents[0]
	}
	choice := strings.TrimSpace(line)
	switch choice {
	case "":
		return agents[0]
	case "0":
		return None
	}
	if n, err := strconv.Atoi(choice); err == nil {
		if n >= 1 && n <= len(agents) {
			return agents[n-1]
		}
		return None
	}
	if Available(choice) {
		return choice
	}
	return None
}

func stdinIsTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
