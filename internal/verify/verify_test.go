package verify

import (
	"strings"
	"testing"
)

// fakeRunner returns scripted output/exit per command.
type fakeRunner struct {
	results map[string][2]any // command -> [output(string), exit(int)]
	calls   []string
	dirs    []string
}

func (f *fakeRunner) Run(dir, command string) (string, int) {
	f.calls = append(f.calls, command)
	f.dirs = append(f.dirs, dir)
	if r, ok := f.results[command]; ok {
		return r[0].(string), r[1].(int)
	}
	return "", 0
}

func TestRunAllPass(t *testing.T) {
	fr := &fakeRunner{results: map[string][2]any{
		"go test ./...": {"ok", 0},
		"make lint":     {"clean", 0},
	}}
	r := Run("/work", []string{"go test ./...", "make lint"}, fr)
	if !r.Passed() {
		t.Fatalf("expected pass, got %v", r.Summary())
	}
	if len(r.Commands) != 2 {
		t.Fatalf("want 2 commands, got %d", len(r.Commands))
	}
	// commands run in the given dir.
	for _, d := range fr.dirs {
		if d != "/work" {
			t.Fatalf("command ran in %q, want /work", d)
		}
	}
}

func TestRunOneFailsStillRunsRest(t *testing.T) {
	fr := &fakeRunner{results: map[string][2]any{
		"a": {"", 1},
		"b": {"", 0},
	}}
	r := Run("/w", []string{"a", "b"}, fr)
	if r.Passed() {
		t.Fatal("run with a failing command should not pass")
	}
	if len(fr.calls) != 2 {
		t.Fatalf("both commands should run (evidence complete), got %v", fr.calls)
	}
}

func TestRunEmptyDoesNotPass(t *testing.T) {
	r := Run("/w", nil, &fakeRunner{})
	if r.Passed() {
		t.Fatal("empty verification must not pass — nothing verified it")
	}
}

func TestRunSkipsBlankCommands(t *testing.T) {
	fr := &fakeRunner{}
	r := Run("/w", []string{"", "  ", "echo hi"}, fr)
	if len(r.Commands) != 1 || len(fr.calls) != 1 {
		t.Fatalf("blank commands should be skipped, ran %v", fr.calls)
	}
}

func TestMarkdownEvidence(t *testing.T) {
	fr := &fakeRunner{results: map[string][2]any{
		"go test": {"PASS\nok pkg", 0},
		"lint":    {"1 issue", 3},
	}}
	r := Run("/repo", []string{"go test", "lint"}, fr)
	md := r.Markdown()
	if !strings.Contains(md, "# Verification — FAILED") {
		t.Fatalf("evidence header wrong:\n%s", md)
	}
	if !strings.Contains(md, "`go test` (exit 0)") || !strings.Contains(md, "`lint` (exit 3)") {
		t.Fatalf("command results missing:\n%s", md)
	}
	if !strings.Contains(md, "ok pkg") || !strings.Contains(md, "1 issue") {
		t.Fatalf("output not embedded:\n%s", md)
	}
	if !strings.Contains(md, "✓") || !strings.Contains(md, "✗") {
		t.Fatalf("pass/fail marks missing:\n%s", md)
	}
}

func TestSummary(t *testing.T) {
	fr := &fakeRunner{results: map[string][2]any{"a": {"", 0}, "b": {"", 1}}}
	r := Run("/w", []string{"a", "b"}, fr)
	if got := r.Summary(); got != "FAILED (1/2 commands passed)" {
		t.Fatalf("summary = %q", got)
	}
}
