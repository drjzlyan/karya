package agentrun

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/drjzlyan/karya/internal/spec"
)

// captureExec returns an execFunc that records its calls and answers with a
// canned reply, plus a handle to the recorded calls.
func captureExec(reply string) (execFunc, *[][]string) {
	var calls [][]string
	return func(_ context.Context, dir string, argv, extraEnv []string) (string, error) {
		calls = append(calls, append([]string{"dir=" + dir}, append(argv, extraEnv...)...))
		return reply, nil
	}, &calls
}

// swapExec points defaultExec at fn for the duration of a test.
func swapExec(t *testing.T, fn execFunc) {
	t.Helper()
	old := defaultExec
	defaultExec = fn
	t.Cleanup(func() { defaultExec = old })
}

func testSpec() *spec.Spec {
	return &spec.Spec{
		ID:           "2026-08-06-add-retry",
		Objective:    "Retry transient download failures.",
		Criteria:     []spec.Criterion{{Text: "5xx retried 3 times"}},
		Verification: []string{"go test ./internal/tools/"},
	}
}

func TestForUnknownAgent(t *testing.T) {
	if _, err := For("emacs"); !errors.Is(err, ErrUnknownAgent) {
		t.Errorf("For(emacs) err = %v, want ErrUnknownAgent", err)
	}
	if Supported("emacs") {
		t.Error("Supported(emacs) = true")
	}
	if !Supported("shell") || !Supported("claude") {
		t.Error("Supported should cover shell and registered CLIs")
	}
}

func TestCapsMatrixCoversEveryAdapter(t *testing.T) {
	m := CapsMatrix()
	for _, name := range append(Names(), "shell") {
		if _, ok := m[name]; !ok {
			t.Errorf("CapsMatrix missing %s", name)
		}
	}
	if !m["claude"].PlanMode || !m["codex"].PlanMode {
		t.Error("claude and codex have native plan modes")
	}
	for _, name := range []string{"crush", "gemini", "aider", "copilot", "shell"} {
		if m[name].PlanMode {
			t.Errorf("%s has no native plan mode; emulation only", name)
		}
	}
	if shell := m["shell"]; !shell.Headless || shell.Resume || shell.Skills || shell.MCP || shell.Streaming {
		t.Errorf("shell adapter is degraded mode: Headless only, got %+v", shell)
	}
}

func TestNamesMatchesPreferenceOrder(t *testing.T) {
	got := strings.Join(Names(), ",")
	want := "crush,claude,codex,gemini,aider,copilot"
	if got != want {
		t.Errorf("Names = %s, want %s", got, want)
	}
}

func TestDetectFindsBinariesInPreferenceOrder(t *testing.T) {
	if os.PathListSeparator != ':' {
		t.Skip("POSIX PATH manipulation")
	}
	bin := t.TempDir()
	for _, name := range []string{"claude", "aider"} {
		p := filepath.Join(bin, name)
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	got := Detect()
	if len(got) != 2 || got[0] != "claude" || got[1] != "aider" {
		t.Errorf("Detect = %v, want [claude aider]", got)
	}
}

func TestAdapterArgvContracts(t *testing.T) {
	cases := []struct {
		name         string
		planPrefix   []string // expected argv prefix for the plan step
		implPrefix   []string // expected argv prefix for the implement step
		planSameArgv bool     // emulation: plan reuses the one-shot argv
	}{
		{"claude", []string{"claude", "-p", "--permission-mode", "plan"}, []string{"claude", "-p"}, false},
		{"codex", []string{"codex", "exec", "--sandbox", "read-only"}, []string{"codex", "exec"}, false},
		{"crush", nil, []string{"crush", "run"}, true},
		{"gemini", nil, []string{"gemini", "-p"}, true},
		{"aider", nil, []string{"aider", "--yes-always", "--no-auto-commits", "-m"}, true},
		{"copilot", nil, []string{"copilot", "--allow-all-tools", "-p"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			exec, calls := captureExec("ok")
			swapExec(t, exec)
			a, err := For(c.name)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := a.Implement(context.Background(), "/wt", "PROMPT"); err != nil {
				t.Fatal(err)
			}
			if _, err := a.Plan(context.Background(), "/wt", "PROMPT"); err != nil {
				t.Fatal(err)
			}
			if len(*calls) != 2 {
				t.Fatalf("expected 2 exec calls, got %v", *calls)
			}
			impl, plan := (*calls)[0], (*calls)[1]
			assertPrefix(t, "implement", impl, append([]string{"dir=/wt"}, c.implPrefix...))
			if c.planSameArgv {
				// Emulation: same argv as implement; the prompt carries the
				// read-only scaffold, and the prompt is the last argument.
				assertPrefix(t, "plan", plan, append([]string{"dir=/wt"}, c.implPrefix...))
			} else {
				assertPrefix(t, "plan", plan, append([]string{"dir=/wt"}, c.planPrefix...))
			}
			if plan[len(plan)-1] != "PROMPT" {
				t.Errorf("plan argv should end with the prompt, got %v", plan)
			}
		})
	}
}

// assertPrefix checks that got begins with want.
func assertPrefix(t *testing.T, what string, got, want []string) {
	t.Helper()
	if len(got) < len(want) {
		t.Fatalf("%s argv %v shorter than expected prefix %v", what, got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("%s argv[%d] = %q, want %q (full: %v)", what, i, got[i], w, got)
		}
	}
}

func TestShellAdapterRequiresCommand(t *testing.T) {
	t.Setenv(shellEnv, "")
	a, err := For("shell")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Implement(context.Background(), "/wt", "PROMPT"); err == nil ||
		!strings.Contains(err.Error(), shellEnv) {
		t.Errorf("expected a %s error, got %v", shellEnv, err)
	}
}

func TestShellAdapterPassesPromptFile(t *testing.T) {
	t.Setenv(shellEnv, "my-agent --run")
	var gotArgv, gotEnv []string
	var gotDir string
	swapExec(t, func(_ context.Context, dir string, argv, extraEnv []string) (string, error) {
		gotDir, gotArgv, gotEnv = dir, argv, extraEnv
		// The prompt file must exist and carry the prompt by the time the
		// command runs.
		path := strings.TrimPrefix(extraEnv[0], promptFileEnv+"=")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("prompt file unreadable during exec: %v", err)
		}
		if string(data) != "PROMPT BODY" {
			t.Errorf("prompt file = %q, want PROMPT BODY", data)
		}
		return "done", nil
	})
	a, err := For("shell")
	if err != nil {
		t.Fatal(err)
	}
	out, err := a.Plan(context.Background(), "/wt", "PROMPT BODY")
	if err != nil {
		t.Fatal(err)
	}
	if out != "done" {
		t.Errorf("output = %q", out)
	}
	if gotDir != "/wt" {
		t.Errorf("dir = %q", gotDir)
	}
	if len(gotArgv) != 3 || gotArgv[0] != "sh" || gotArgv[1] != "-c" || gotArgv[2] != "my-agent --run" {
		t.Errorf("argv = %v", gotArgv)
	}
	if len(gotEnv) != 1 || !strings.HasPrefix(gotEnv[0], promptFileEnv+"=") {
		t.Errorf("env = %v", gotEnv)
	}
}

func TestResolveAgentPrecedence(t *testing.T) {
	s := testSpec()
	s.Agent = "claude"
	s.Agents = map[string]string{"plan": "codex"}

	// Explicit request wins over every pin.
	if name, err := resolveAgent(Request{Step: StepPlan, Agent: "gemini", Spec: s}); err != nil || name != "gemini" {
		t.Errorf("explicit: %q, %v", name, err)
	}
	// Per-step pin beats the spec's preferred agent.
	if name, err := resolveAgent(Request{Step: StepPlan, Spec: s}); err != nil || name != "codex" {
		t.Errorf("step pin: %q, %v", name, err)
	}
	// A step without a pin falls to the spec agent.
	if name, err := resolveAgent(Request{Step: StepImplement, Spec: s}); err != nil || name != "claude" {
		t.Errorf("spec agent: %q, %v", name, err)
	}
	// An unsupported explicit name is an error, not a silent fallback.
	if _, err := resolveAgent(Request{Step: StepPlan, Agent: "emacs", Spec: s}); !errors.Is(err, ErrUnknownAgent) {
		t.Errorf("unsupported: %v, want ErrUnknownAgent", err)
	}
}

func TestResolveAgentNoAgentsAnywhere(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // nothing detected
	_, err := resolveAgent(Request{Step: StepPlan, Spec: testSpec()})
	if err == nil || !strings.Contains(err.Error(), "no coding agent") {
		t.Errorf("err = %v, want a no-agent error", err)
	}
}

func runStepRequest(t *testing.T) Request {
	t.Helper()
	dir := t.TempDir()
	return Request{
		Step:     StepPlan,
		TaskID:   "2026-08-06-add-retry",
		Agent:    "claude",
		Worktree: filepath.Join(dir, "wt"),
		TaskDir:  filepath.Join(dir, "task"),
		RepoRoot: filepath.Join(dir, "repo"),
		Spec:     testSpec(),
	}
}

func TestRunStepPlanWritesPlanAndTranscript(t *testing.T) {
	if err := os.MkdirAll(runStepRequest(t).TaskDir, 0o755); err != nil {
		t.Fatal(err)
	}
	req := runStepRequest(t)
	for _, d := range []string{req.TaskDir, req.Worktree, req.RepoRoot} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	exec, calls := captureExec("```markdown\n# Plan\n\n1. add retry loop\n```\n")
	swapExec(t, exec)

	out, err := RunStep(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if out.Agent != "claude" {
		t.Errorf("agent = %q", out.Agent)
	}
	// PLAN.md holds the sanitized plan (fences stripped).
	plan, err := os.ReadFile(out.PlanPath)
	if err != nil {
		t.Fatalf("read PLAN.md: %v", err)
	}
	if strings.Contains(string(plan), "```") || !strings.Contains(string(plan), "1. add retry loop") {
		t.Errorf("PLAN.md not sanitized:\n%s", plan)
	}
	// The transcript records prompt, output, and the native plan-mode label.
	tr, err := os.ReadFile(out.Transcript)
	if err != nil {
		t.Fatalf("read transcript: %v", err)
	}
	for _, want := range []string{"# Transcript — plan step", "agent: claude", "plan mode: native", "## Prompt", "Retry transient download failures", "## Output", "add retry loop"} {
		if !strings.Contains(string(tr), want) {
			t.Errorf("transcript missing %q:\n%s", want, tr)
		}
	}
	if len(*calls) != 1 {
		t.Fatalf("exec calls = %v", *calls)
	}
}

func TestRunStepPlanEmptyPlanIsAnError(t *testing.T) {
	req := runStepRequest(t)
	for _, d := range []string{req.TaskDir, req.Worktree, req.RepoRoot} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	exec, _ := captureExec("```\n```\n") // fences only → empty after sanitize
	swapExec(t, exec)
	if _, err := RunStep(context.Background(), req); err == nil || !strings.Contains(err.Error(), "empty plan") {
		t.Errorf("err = %v, want an empty-plan error", err)
	}
}

func TestRunStepImplementWritesNoPlan(t *testing.T) {
	req := runStepRequest(t)
	req.Step = StepImplement
	for _, d := range []string{req.TaskDir, req.Worktree, req.RepoRoot} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	exec, _ := captureExec("edits applied")
	swapExec(t, exec)
	out, err := RunStep(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if out.PlanPath != "" {
		t.Errorf("implement step must not write PLAN.md, got %s", out.PlanPath)
	}
	if _, err := os.Stat(out.Transcript); err != nil {
		t.Errorf("transcript missing: %v", err)
	}
}

func TestRunStepFailureStillWritesTranscript(t *testing.T) {
	req := runStepRequest(t)
	for _, d := range []string{req.TaskDir, req.Worktree, req.RepoRoot} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	swapExec(t, func(context.Context, string, []string, []string) (string, error) {
		return "", errors.New("agent exploded")
	})
	_, err := RunStep(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "agent exploded") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(err.Error(), "transcript:") {
		t.Errorf("error should point at the transcript: %v", err)
	}
	matches, _ := filepath.Glob(filepath.Join(req.TaskDir, "transcripts", "*.md"))
	if len(matches) != 1 {
		t.Fatalf("expected one transcript, got %v", matches)
	}
	data, _ := os.ReadFile(matches[0])
	if !strings.Contains(string(data), "error: agent exploded") {
		t.Errorf("failed run not recorded in transcript:\n%s", data)
	}
}

func TestRunStepValidatesRequest(t *testing.T) {
	if _, err := RunStep(context.Background(), Request{}); err == nil {
		t.Error("nil spec should error")
	}
	if _, err := RunStep(context.Background(), Request{Spec: testSpec()}); err == nil ||
		!strings.Contains(err.Error(), "no worktree") {
		t.Errorf("missing worktree should error with guidance, got %v", err)
	}
	req := runStepRequest(t)
	req.Step = "dance"
	if _, err := RunStep(context.Background(), req); err == nil || !strings.Contains(err.Error(), "unsupported step") {
		t.Errorf("unsupported step should error, got %v", err)
	}
}
