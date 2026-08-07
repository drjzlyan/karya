package agentrun

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/drjzlyan/karya/internal/config"
	"github.com/drjzlyan/karya/internal/prompts"
	"github.com/drjzlyan/karya/internal/spec"
)

// Request describes one headless step execution for a task. The caller (the
// CLI; later the TUI) owns state transitions — RunStep only drives the agent
// and records artifacts, keeping the adapter layer free of gate policy.
type Request struct {
	Step     Step       // which step to run (StepPlan or StepImplement)
	TaskID   string     // task id, for transcript naming
	Agent    string     // explicit agent choice; "" resolves spec pins, then PATH
	Worktree string     // the task's isolated worktree — the agent's cwd
	TaskDir  string     // .karya/tasks/<id> — artifacts land here
	RepoRoot string     // repo root, for agent-facing docs (AGENTS.md et al)
	Spec     *spec.Spec // the task contract (required)
	Plan     string     // approved PLAN.md content (implement step)
	Feedback string     // human feedback when this run is a rejection revision
}

// Outcome reports what a step run produced and where it landed.
type Outcome struct {
	Agent      string // the agent that ran (after resolution)
	Output     string // the agent's raw stdout
	Transcript string // path of the transcript file in the task dir
	PlanPath   string // path of PLAN.md (plan step only)
}

// RunStep executes one task step headlessly: resolve the agent (request → spec
// per-step pin → spec agent → first detected), assemble the prompt from the
// spec + repo docs + feedback (internal/prompts), drive the agent in the task
// worktree, and record the artifacts — a transcript of every run and PLAN.md
// for the plan step (DESIGN.md §5).
func RunStep(ctx context.Context, req Request) (Outcome, error) {
	if req.Spec == nil {
		return Outcome{}, fmt.Errorf("agentrun: a spec is required")
	}
	if req.Worktree == "" {
		return Outcome{}, fmt.Errorf("agentrun: task %s has no worktree; run `karya task start %s` first", req.TaskID, req.TaskID)
	}
	name, err := resolveAgent(req)
	if err != nil {
		return Outcome{}, err
	}
	a, err := For(name)
	if err != nil {
		return Outcome{}, err
	}

	// Layered agent context: global (user-wide) → project (repo docs) → task
	// (MEMORY.md). See internal/prompts (DESIGN.md §5).
	ctxDoc := prompts.Context(config.Resolve().GlobalInstructions(), req.RepoRoot, req.TaskDir)
	var prompt string
	switch req.Step {
	case StepPlan:
		prompt = prompts.Plan(prompts.PlanInput{Spec: req.Spec, Context: ctxDoc, Feedback: req.Feedback})
	case StepImplement:
		prompt = prompts.Implement(prompts.ImplementInput{
			Spec: req.Spec, Plan: req.Plan, Context: ctxDoc, Feedback: req.Feedback,
		})
	default:
		return Outcome{}, fmt.Errorf("agentrun: unsupported step %q", req.Step)
	}

	started := time.Now().UTC()
	output, runErr := runOn(ctx, a, req.Step, req.Worktree, prompt)
	finished := time.Now().UTC()

	out := Outcome{Agent: name, Output: output}
	transcript, werr := writeTranscript(req, a, prompt, output, runErr, started, finished)
	if werr != nil {
		return out, werr
	}
	out.Transcript = transcript
	if runErr != nil {
		return out, fmt.Errorf("agentrun: %s step %s failed (transcript: %s): %w", req.TaskID, req.Step, transcript, runErr)
	}

	if req.Step == StepPlan {
		plan := SanitizeMessage(output)
		if plan == "" {
			return out, fmt.Errorf("agentrun: %s produced an empty plan (transcript: %s)", name, transcript)
		}
		planPath := filepath.Join(req.TaskDir, "PLAN.md")
		if err := os.WriteFile(planPath, []byte(plan+"\n"), 0o644); err != nil {
			return out, fmt.Errorf("agentrun: write PLAN.md: %w", err)
		}
		out.PlanPath = planPath
	}
	return out, nil
}

// runOn dispatches the step to the adapter's matching method.
func runOn(ctx context.Context, a Agent, step Step, dir, prompt string) (string, error) {
	if step == StepPlan {
		return a.Plan(ctx, dir, prompt)
	}
	return a.Implement(ctx, dir, prompt)
}

// resolveAgent picks the agent for a step: the explicit request choice wins,
// then the spec's per-step pin, then the spec's preferred agent, then the
// first detected CLI (DESIGN.md §5 — mix-and-match steps).
func resolveAgent(req Request) (string, error) {
	candidates := []string{req.Agent, req.Spec.Agents[string(req.Step)], req.Spec.Agent}
	for _, name := range candidates {
		if name == "" {
			continue
		}
		if !Supported(name) {
			return "", fmt.Errorf("agentrun: %w: %q", ErrUnknownAgent, name)
		}
		return name, nil
	}
	if detected := Detect(); len(detected) > 0 {
		return detected[0], nil
	}
	return "", fmt.Errorf("agentrun: no coding agent detected on PATH; install one of %s, or set %s for the shell adapter",
		joinNames(Names()), shellEnv)
}

// writeTranscript records one run as a Markdown artifact in the task dir:
// who ran, with what capabilities, the exact prompt, and the raw output (or
// error). Artifacts over chat logs (DESIGN.md §1) — transcripts are how a
// human (or a reviewer agent) audits what an agent was asked and answered.
func writeTranscript(req Request, a Agent, prompt, output string, runErr error, started, finished time.Time) (string, error) {
	dir := filepath.Join(req.TaskDir, "transcripts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("agentrun: create transcripts dir: %w", err)
	}
	path := filepath.Join(dir, fmt.Sprintf("%s-%s-%s.md",
		req.Step, a.Name(), started.Format("20060102-150405")))

	planMode := "prompt-scaffold emulation"
	if a.Caps().PlanMode {
		planMode = "native"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Transcript — %s step\n\n", req.Step)
	fmt.Fprintf(&b, "- task: %s\n- agent: %s\n- plan mode: %s\n- started: %s\n- finished: %s\n",
		req.TaskID, a.Name(), planMode,
		started.Format(time.RFC3339), finished.Format(time.RFC3339))
	if runErr != nil {
		fmt.Fprintf(&b, "- error: %v\n", runErr)
	}
	b.WriteString("\n## Prompt\n\n")
	b.WriteString(prompt)
	b.WriteString("\n\n## Output\n\n")
	b.WriteString(output)
	b.WriteString("\n")

	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return "", fmt.Errorf("agentrun: write transcript: %w", err)
	}
	return path, nil
}
