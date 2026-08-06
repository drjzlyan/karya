package agentrun

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Runner executes external commands in a working directory. Output captures
// stdout for inspection (status, diff); Run inherits the caller's stdio for
// interactive/streaming commands (commit, push). Defined here so callers can
// substitute a fake in tests.
type Runner interface {
	Output(dir, name string, args ...string) (string, error)
	Run(dir, name string, args ...string) error
}

// ExecRunner is the production Runner: it shells out to real commands. Output
// captures stdout; Run streams stdio so interactive git hooks and gh prompts
// work.
type ExecRunner struct{}

// Output runs name in dir and returns its trimmed stdout. On failure it folds
// the command's stderr into the error so callers surface git's own diagnostic
// (e.g. "not a git repository") instead of a bare "exit status 128".
func (ExecRunner) Output(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), msg)
		}
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// Run runs name in dir with the caller's stdio attached.
func (ExecRunner) Run(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
