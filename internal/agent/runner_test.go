package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCLIRunnerName(t *testing.T) {
	if got := NewCLIRunner("claude").Name(); got != "claude" {
		t.Errorf("Name() = %q, want claude", got)
	}
}

func TestCLIRunnerInteractiveCommand(t *testing.T) {
	tests := []struct {
		name    string
		agent   string
		wantCmd string
		wantOK  bool
	}{
		{"known agent", "claude", "claude", true},
		{"none is shell-only", None, "", false},
		{"empty is shell-only", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, ok := NewCLIRunner(tt.agent).InteractiveCommand()
			if cmd != tt.wantCmd || ok != tt.wantOK {
				t.Errorf("InteractiveCommand() = (%q, %v), want (%q, %v)", cmd, ok, tt.wantCmd, tt.wantOK)
			}
		})
	}
}

func TestCLIRunnerHeadlessNoMode(t *testing.T) {
	// aider has no known one-shot mode; Headless must report ErrNoHeadless so
	// callers fall back to the interactive pane rather than inventing a flag.
	_, err := NewCLIRunner("aider").Headless(context.Background(), t.TempDir(), "hi")
	if !errors.Is(err, ErrNoHeadless) {
		t.Errorf("Headless(aider) error = %v, want ErrNoHeadless", err)
	}
}

func TestCLIRunnerHeadlessRun(t *testing.T) {
	// White-box: inject a fake exec so the test stays hermetic (no real agent).
	var gotDir string
	var gotArgv []string
	r := &cliRunner{
		name: "claude",
		run: func(_ context.Context, dir string, argv []string) (string, error) {
			gotDir, gotArgv = dir, argv
			return "feat: do the thing\n", nil
		},
	}

	out, err := r.Headless(context.Background(), "/work", "author a message")
	if err != nil {
		t.Fatalf("Headless() error = %v", err)
	}
	if strings.TrimSpace(out) != "feat: do the thing" {
		t.Errorf("Headless() out = %q", out)
	}
	if gotDir != "/work" {
		t.Errorf("dir forwarded = %q, want /work", gotDir)
	}
	// The prompt must reach the agent as the final argv element (claude uses -p).
	if len(gotArgv) == 0 || gotArgv[len(gotArgv)-1] != "author a message" {
		t.Errorf("argv = %v, want prompt as last element", gotArgv)
	}
}
