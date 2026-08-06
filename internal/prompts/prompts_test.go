package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/drjzlyan/karya/internal/spec"
)

func testSpec() *spec.Spec {
	return &spec.Spec{
		ID:        "2026-08-06-add-retry",
		Status:    "draft",
		Agent:     "claude",
		Objective: "Retry transient download failures so flaky networks do not break installs.",
		Criteria: []spec.Criterion{
			{Text: "`download` retries transient 5xx up to 3 times with backoff"},
			{Text: "permanent failures (4xx) never retry"},
		},
		Context:      "internal/tools/download.go owns the HTTP path.",
		Constraints:  "No new dependencies.",
		Verification: []string{"go test ./internal/tools/ -run Retry"},
	}
}

func TestPlanPromptCarriesContract(t *testing.T) {
	p := Plan(PlanInput{Spec: testSpec()})
	for _, want := range []string{
		"planning agent",
		"2026-08-06-add-retry",
		"Retry transient download failures",
		"retries transient 5xx up to 3 times",
		"No new dependencies.",
		"go test ./internal/tools/ -run Retry",
		"Do NOT modify",
		"read-only",
		"Risks & assumptions",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("plan prompt missing %q:\n%s", want, p)
		}
	}
}

func TestPlanPromptOmitsEmptyOptionalSections(t *testing.T) {
	p := Plan(PlanInput{Spec: testSpec()})
	if strings.Contains(p, "Repository docs") {
		t.Error("plan prompt should omit the repo-docs section when there is no context")
	}
	if strings.Contains(p, "Human feedback") {
		t.Error("plan prompt should omit the feedback section on a first run")
	}
}

func TestPlanPromptRevisionIncludesFeedback(t *testing.T) {
	p := Plan(PlanInput{Spec: testSpec(), Feedback: "step 2 skips the backoff cap\nadd it"})
	for _, want := range []string{"Human feedback", "REJECTED at the plan gate", "> step 2 skips the backoff cap\n> add it"} {
		if !strings.Contains(p, want) {
			t.Errorf("revision prompt missing %q:\n%s", want, p)
		}
	}
}

func TestImplementPromptCarriesPlanAndRules(t *testing.T) {
	p := Implement(ImplementInput{
		Spec: testSpec(),
		Plan: "# Plan\n\n1. add retry loop\n",
	})
	for _, want := range []string{
		"implementation agent",
		"approved plan",
		"1. add retry loop",
		"isolated git worktree",
		"EVERY acceptance criterion",
		"agents never merge",
		"Do NOT run git commit",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("implement prompt missing %q:\n%s", want, p)
		}
	}
	// Without feedback there is no rejection lead-in beyond the rules block.
	if strings.Contains(p, "The human wrote:") {
		t.Error("implement prompt should not reference feedback when none was given")
	}
}

func TestImplementPromptRevisionIncludesFeedback(t *testing.T) {
	p := Implement(ImplementInput{Spec: testSpec(), Plan: "plan", Feedback: "4xx must not retry"})
	if !strings.Contains(p, "REJECTED at the diff gate") || !strings.Contains(p, "> 4xx must not retry") {
		t.Errorf("implement revision prompt missing feedback:\n%s", p)
	}
}

func TestRepoContextReadsLabeledDocs(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# Contract\nrun: make gate\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	karyaDir := filepath.Join(root, ".karya")
	if err := os.MkdirAll(karyaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(karyaDir, "CONTEXT.md"), []byte("project context here"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := RepoContext(root)
	for _, want := range []string{"### AGENTS.md", "make gate", "### .karya/CONTEXT.md", "project context here"} {
		if !strings.Contains(ctx, want) {
			t.Errorf("RepoContext missing %q:\n%s", want, ctx)
		}
	}
}

func TestRepoContextMissingIsEmpty(t *testing.T) {
	if got := RepoContext(t.TempDir()); got != "" {
		t.Errorf("RepoContext = %q, want empty", got)
	}
}

func TestRepoContextTruncatesHugeDocs(t *testing.T) {
	root := t.TempDir()
	huge := strings.Repeat("x", maxRepoDocBytes+100)
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(huge), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := RepoContext(root)
	if !strings.Contains(ctx, "(truncated)") {
		t.Error("huge AGENTS.md should be marked truncated")
	}
	if len(ctx) > maxRepoDocBytes+1024 {
		t.Errorf("RepoContext not capped: %d bytes", len(ctx))
	}
}

func TestContextLayersGlobalProjectTask(t *testing.T) {
	dir := t.TempDir()
	global := filepath.Join(dir, "global.md")
	os.WriteFile(global, []byte("GLOBAL RULES"), 0o644)
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("PROJECT RULES"), 0o644)
	taskDir := filepath.Join(dir, "task")
	os.MkdirAll(taskDir, 0o755)
	os.WriteFile(filepath.Join(taskDir, "MEMORY.md"), []byte("TASK MEMORY"), 0o644)

	ctx := Context(global, dir, taskDir)
	for _, want := range []string{"Global instructions", "GLOBAL RULES", "Project instructions", "PROJECT RULES", "Task memory", "TASK MEMORY"} {
		if !strings.Contains(ctx, want) {
			t.Fatalf("context missing %q:\n%s", want, ctx)
		}
	}
	// Order: global before project before task.
	gi, pi, ti := strings.Index(ctx, "GLOBAL"), strings.Index(ctx, "PROJECT"), strings.Index(ctx, "TASK MEMORY")
	if gi >= pi || pi >= ti {
		t.Fatalf("layers out of order (g=%d p=%d t=%d):\n%s", gi, pi, ti, ctx)
	}
}

func TestContextOverrideDropsOuterLayers(t *testing.T) {
	dir := t.TempDir()
	global := filepath.Join(dir, "global.md")
	os.WriteFile(global, []byte("GLOBAL RULES"), 0o644)
	// Project marks override -> global is dropped.
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("PROJECT RULES <!-- karya:override -->"), 0o644)

	ctx := Context(global, dir, "")
	if strings.Contains(ctx, "GLOBAL RULES") {
		t.Fatalf("override should drop the global layer:\n%s", ctx)
	}
	if !strings.Contains(ctx, "PROJECT RULES") {
		t.Fatalf("override layer should remain:\n%s", ctx)
	}
}

func TestContextEmptyWhenNothing(t *testing.T) {
	if ctx := Context("", t.TempDir(), ""); ctx != "" {
		t.Fatalf("empty context expected, got %q", ctx)
	}
}
