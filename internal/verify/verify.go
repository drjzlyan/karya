// Package verify runs a task spec's Verification commands in the task worktree
// and turns the result into evidence — karya certifies; the agent never
// self-certifies (DESIGN.md §3, §7). Each command's exit code and output are
// captured into a VERIFY-<n>.md report the human reviews at the verify gate.
package verify

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Runner executes a shell command line in a directory, returning its combined
// output and exit code. It is an interface so tests can substitute a fake.
type Runner interface {
	Run(dir, command string) (output string, exitCode int)
}

// ShellRunner runs commands via `sh -c` with combined stdout/stderr.
type ShellRunner struct{}

// Run executes command in dir and returns combined output and the exit code
// (non-zero, including 127 for a missing shell).
func (ShellRunner) Run(dir, command string) (string, int) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), exitCodeOf(err)
}

func exitCodeOf(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return 1
}

// CommandResult is the outcome of one Verification command.
type CommandResult struct {
	Command  string
	Output   string
	ExitCode int
}

// Passed reports whether the command exited zero.
func (c CommandResult) Passed() bool { return c.ExitCode == 0 }

// Result is the outcome of a full verification run.
type Result struct {
	Dir      string
	Commands []CommandResult
	When     time.Time
}

// Passed reports whether every command passed (an empty run does not pass —
// there is nothing verifying it).
func (r Result) Passed() bool {
	if len(r.Commands) == 0 {
		return false
	}
	for _, c := range r.Commands {
		if !c.Passed() {
			return false
		}
	}
	return true
}

// Run executes each command in dir (in order) and collects the results. It does
// not stop on failure — every command runs so the evidence is complete.
func Run(dir string, commands []string, runner Runner) Result {
	if runner == nil {
		runner = ShellRunner{}
	}
	res := Result{Dir: dir, When: time.Now().UTC()}
	for _, c := range commands {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		out, code := runner.Run(dir, c)
		res.Commands = append(res.Commands, CommandResult{Command: c, Output: out, ExitCode: code})
	}
	return res
}

// Markdown renders the result as a VERIFY-<n>.md evidence report.
func (r Result) Markdown() string {
	var b strings.Builder
	status := "FAILED"
	if r.Passed() {
		status = "PASSED"
	}
	fmt.Fprintf(&b, "# Verification — %s\n\n", status)
	fmt.Fprintf(&b, "- when: %s\n", r.When.Format(time.RFC3339))
	fmt.Fprintf(&b, "- workdir: %s\n\n", r.Dir)
	for _, c := range r.Commands {
		mark := "✗"
		if c.Passed() {
			mark = "✓"
		}
		fmt.Fprintf(&b, "## %s `%s` (exit %d)\n\n", mark, c.Command, c.ExitCode)
		out := strings.TrimRight(c.Output, "\n")
		if out != "" {
			b.WriteString("```\n")
			b.WriteString(out)
			b.WriteString("\n```\n\n")
		}
	}
	return b.String()
}

// Summary is a one-line result for the CLI.
func (r Result) Summary() string {
	passed := 0
	for _, c := range r.Commands {
		if c.Passed() {
			passed++
		}
	}
	verdict := "FAILED"
	if r.Passed() {
		verdict = "PASSED"
	}
	return fmt.Sprintf("%s (%d/%d commands passed)", verdict, passed, len(r.Commands))
}
