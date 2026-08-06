package agent

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrNoHeadless reports that a Runner has no non-interactive (one-shot) mode.
// Callers that receive it fall back to the interactive agent pane rather than
// inventing a flag that could produce garbage output.
var ErrNoHeadless = errors.New("agent has no headless mode")

// Runner is the pluggable agent engine karya drives. It abstracts over the way
// an agent is invoked so the rest of karya depends on this interface, not on a
// specific CLI. Today the only implementation wraps a BYO agent CLI (cliRunner);
// a native Claude-API engine is designed to drop in behind the same interface
// (ROADMAP Phase 13) without changing any consumer.
//
// The interface is intentionally small and defined by its consumers (session/
// agent management for the interactive mode, ship/task authoring for headless),
// per the interface-segregation and dependency-inversion rules in AGENTS.md.
type Runner interface {
	// Name is the agent identifier, e.g. "claude" or the None sentinel.
	Name() string

	// InteractiveCommand returns the shell command that starts the agent's
	// interactive session in a pane, and ok=false for None/shell-only, so the
	// caller opens a bare shell instead.
	InteractiveCommand() (cmd string, ok bool)

	// Headless runs the agent non-interactively in dir with prompt and returns
	// its raw stdout. It returns ErrNoHeadless when this engine has no one-shot
	// mode. Sanitizing the reply is the caller's job (it is task-specific).
	Headless(ctx context.Context, dir, prompt string) (string, error)
}

// cliRunner wraps a detected BYO agent CLI (claude/codex/…) as a Runner. run is
// the exec seam, overridable in tests to keep them hermetic.
type cliRunner struct {
	name string
	run  func(ctx context.Context, dir string, argv []string) (string, error)
}

// NewCLIRunner returns a Runner backed by the named agent CLI.
func NewCLIRunner(name string) Runner {
	return &cliRunner{name: name, run: execHeadless}
}

// Name implements Runner.
func (r *cliRunner) Name() string { return r.name }

// InteractiveCommand implements Runner: for a CLI, the command is just the agent
// binary name; None and the empty sentinel are shell-only.
func (r *cliRunner) InteractiveCommand() (string, bool) {
	if r.name == "" || r.name == None {
		return "", false
	}
	return r.name, true
}

// Headless implements Runner using the agent's documented one-shot invocation
// (headlessArgv). Agents without one return ErrNoHeadless.
func (r *cliRunner) Headless(ctx context.Context, dir, prompt string) (string, error) {
	argv, ok := headlessArgv(r.name, prompt)
	if !ok {
		return "", ErrNoHeadless
	}
	return r.run(ctx, dir, argv)
}

// execHeadless is the real exec seam: it verifies the binary is on PATH, runs it
// in dir, and returns stdout, wrapping stderr into the error on failure so the
// caller can explain the fallback.
func execHeadless(ctx context.Context, dir string, argv []string) (string, error) {
	if _, err := exec.LookPath(argv[0]); err != nil {
		return "", fmt.Errorf("agent %q not on PATH: %w", argv[0], err)
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("agent %q failed: %s: %w", argv[0], msg, err)
		}
		return "", fmt.Errorf("agent %q failed: %w", argv[0], err)
	}
	return string(out), nil
}
