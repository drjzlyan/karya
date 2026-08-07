//go:build integration

package verify

import (
	"strings"
	"testing"
)

func TestShellRunnerRealCommands(t *testing.T) {
	dir := t.TempDir()
	r := Run(dir, []string{
		"echo hello",
		"exit 3",
		"pwd",
	}, ShellRunner{})

	if r.Passed() {
		t.Fatal("run with `exit 3` should fail")
	}
	if len(r.Commands) != 3 {
		t.Fatalf("want 3 commands, got %d", len(r.Commands))
	}
	if !strings.Contains(r.Commands[0].Output, "hello") || r.Commands[0].ExitCode != 0 {
		t.Fatalf("echo result wrong: %+v", r.Commands[0])
	}
	if r.Commands[1].ExitCode != 3 {
		t.Fatalf("exit code not captured: %+v", r.Commands[1])
	}
	// `pwd` should run in the worktree dir.
	if !strings.Contains(r.Commands[2].Output, dir) {
		t.Fatalf("command did not run in dir %q: %q", dir, r.Commands[2].Output)
	}
}
