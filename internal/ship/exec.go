package ship

import (
	"os"
	"os/exec"
	"strings"
)

// ExecRunner is the production Runner: it shells out to real commands. Output
// captures stdout; Run streams stdio so interactive git hooks and gh prompts
// work.
type ExecRunner struct{}

// Output runs name in dir and returns its trimmed stdout.
func (ExecRunner) Output(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimRight(string(out), "\n"), err
}

// Run runs name in dir with the caller's stdio attached.
func (ExecRunner) Run(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
