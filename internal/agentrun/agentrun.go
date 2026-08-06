// Package agentrun is karya's agent adapter layer (DESIGN.md §5): one
// normalized interface drives every coding-agent CLI, headless-first. Each
// agent CLI speaks a different dialect — launch flags, headless mode,
// plan-mode support — and agentrun normalizes them behind the Agent interface
// so the task workflow (karya plan / implement) is agent-agnostic and agents
// can be mixed across steps (plan with one, implement with another).
//
// The capability matrix (Caps) is surfaced rather than hidden: an agent
// without a native plan mode gets plan emulation (a read-only prompt scaffold)
// and callers can see the difference. Every run captures a transcript into the
// task directory — artifacts over chat logs (DESIGN.md §1).
//
// The package also owns the deterministic git plumbing behind task execution
// (Git + ExecRunner, folded in from v0's internal/ship) and commit-message
// authoring (BuildCommitPrompt/SanitizeMessage). Pane-level agent switching
// stays in internal/agent; agentrun is the headless task-work seam.
package agentrun

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sort"
)

// Step is one agent-driven stage of the task workflow (DESIGN.md §2). Review
// and verify steps arrive with Phases C–D; the interface is designed to admit
// them without change.
type Step string

// The steps Phase B drives.
const (
	StepPlan      Step = "plan"
	StepImplement Step = "implement"
)

// Caps is the capability matrix of one agent CLI (DESIGN.md §5). karya works
// at each agent's level instead of the lowest common denominator: PlanMode
// false means the plan step runs as prompt-scaffold emulation (a read-only
// instruction) rather than a CLI-enforced read-only mode.
type Caps struct {
	PlanMode  bool // native read-only plan mode (vs prompt-scaffold emulation)
	Headless  bool // documented non-interactive one-shot mode
	Resume    bool // can resume/continue a previous session
	Skills    bool // supports SKILL.md-style skill packages
	MCP       bool // supports MCP servers
	Streaming bool // can stream output incrementally
}

// Agent is the normalized interface every coding-agent CLI implements. It is
// deliberately small and defined by its consumer (the task step runner), per
// the interface-segregation rule in AGENTS.md; review/verify methods join it
// when their consumers land (Phases C–D).
type Agent interface {
	// Name is the agent identifier, e.g. "claude".
	Name() string
	// Caps reports the agent's capability matrix.
	Caps() Caps
	// Plan runs the plan step headlessly in dir and returns the agent's raw
	// stdout (the plan text). Adapters with a native plan mode enforce
	// read-only behavior via CLI flags; the prompt scaffold carries the same
	// instruction either way.
	Plan(ctx context.Context, dir, prompt string) (string, error)
	// Implement runs the implement step headlessly in dir (the task worktree),
	// returning the agent's raw stdout.
	Implement(ctx context.Context, dir, prompt string) (string, error)
}

// ErrUnknownAgent reports an agent name with no registered adapter.
var ErrUnknownAgent = errors.New("unknown agent")

// order lists the CLI adapters in preference order (matching v0's agent.Known)
// so "first detected" picks the same agent everywhere in karya.
var order = []string{"crush", "claude", "codex", "gemini", "aider", "copilot"}

// registry builds the CLI adapter for a name. The generic shell adapter is not
// here: it is constructed on demand by For (any command can back it).
var registry = map[string]func() Agent{
	"claude":  func() Agent { return claude() },
	"codex":   func() Agent { return codex() },
	"crush":   func() Agent { return crush() },
	"gemini":  func() Agent { return gemini() },
	"aider":   func() Agent { return aider() },
	"copilot": func() Agent { return copilot() },
}

// For returns the Agent for name: a CLI adapter for the known agents, the
// generic shell adapter for "shell", or ErrUnknownAgent.
func For(name string) (Agent, error) {
	if name == "shell" {
		return newShellAdapter(), nil
	}
	if build, ok := registry[name]; ok {
		return build(), nil
	}
	return nil, fmt.Errorf("agentrun: %q: %w (supported: %s, shell)", name, ErrUnknownAgent, joinNames(Names()))
}

// Names returns the CLI adapter names in preference order.
func Names() []string { return append([]string(nil), order...) }

// joinNames renders a name list for error messages.
func joinNames(names []string) string {
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out
}

// Detect returns the adapter names whose CLI is on PATH, in preference order.
// The shell adapter is always available and therefore never "detected".
func Detect() []string {
	var found []string
	for _, name := range order {
		if _, err := exec.LookPath(name); err == nil {
			found = append(found, name)
		}
	}
	return found
}

// Supported reports whether name has an adapter (CLI or shell).
func Supported(name string) bool {
	if name == "shell" {
		return true
	}
	_, ok := registry[name]
	return ok
}

// CapsMatrix returns the capability matrix of every supported agent, keyed by
// name — the data behind surfacing capability gaps (karya doctor, gate
// delegation choices). Keys are sorted for deterministic rendering.
func CapsMatrix() map[string]Caps {
	out := map[string]Caps{}
	names := append(Names(), "shell")
	sort.Strings(names)
	for _, name := range names {
		a, err := For(name)
		if err != nil {
			continue
		}
		out[name] = a.Caps()
	}
	return out
}
