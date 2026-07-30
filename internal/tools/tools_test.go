package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func names(specs []ToolSpec) []string {
	out := make([]string, len(specs))
	for i, s := range specs {
		out[i] = s.Name
	}
	return out
}

func TestPlanIncludesAlwaysOnFirst(t *testing.T) {
	plan := Plan(nil)
	if len(plan) != len(alwaysOn) {
		t.Fatalf("empty selection should plan only always-on servers; got %d", len(plan))
	}
	if plan[0].Name != alwaysOn[0].Name {
		t.Errorf("always-on servers must come first; got %q", plan[0].Name)
	}
}

func TestPlanAppendsPerLanguage(t *testing.T) {
	plan := Plan([]string{"go", "python"})
	got := names(plan)

	// Always-on first, then go's tools, then python's — in that order.
	joined := strings.Join(got, ",")
	if !strings.Contains(joined, "gopls") || !strings.Contains(joined, "basedpyright") {
		t.Fatalf("plan missing language tools: %v", got)
	}
	goIdx := indexOf(got, "gopls")
	pyIdx := indexOf(got, "basedpyright")
	if goIdx == -1 || pyIdx == -1 || goIdx > pyIdx {
		t.Errorf("expected go tools before python tools; got %v", got)
	}
	if goIdx < len(alwaysOn) {
		t.Errorf("language tools should follow always-on servers; got %v", got)
	}
}

func TestPlanIgnoresUnknownLanguage(t *testing.T) {
	if got := Plan([]string{"cobol"}); len(got) != len(alwaysOn) {
		t.Errorf("unknown language should add no tools; got %v", names(got))
	}
}

func TestPlanForSingleLanguage(t *testing.T) {
	rust := PlanFor("rust")
	if len(rust) == 0 || rust[0].Name != "rust-analyzer" {
		t.Errorf("PlanFor(rust) = %v", names(rust))
	}
	if got := PlanFor("nope"); got != nil {
		t.Errorf("PlanFor(unknown) = %v, want nil", got)
	}
}

func TestAvailableDetectsBinInToolsDir(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "gopls"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	in := Installer{ToolsDir: dir, BinDir: bin}
	if !in.available(ToolSpec{Bin: "gopls"}) {
		t.Error("gopls in BinDir should be detected as available")
	}
	if in.available(ToolSpec{Bin: "does-not-exist-xyz"}) {
		t.Error("nonexistent bin should not be available")
	}
}

func TestAvailableDetectsArtifact(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lombok.jar"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	in := Installer{ToolsDir: dir, BinDir: filepath.Join(dir, "bin")}
	if !in.available(ToolSpec{Artifact: "lombok.jar"}) {
		t.Error("existing artifact should be detected as available")
	}
	if in.available(ToolSpec{Artifact: "missing.jar"}) {
		t.Error("missing artifact should not be available")
	}
}

func TestDetectKindReportsMissing(t *testing.T) {
	in := Installer{ToolsDir: t.TempDir(), BinDir: t.TempDir()}
	r := in.one(ToolSpec{Name: "clangd", Bin: "definitely-not-a-real-binary-xyz", Kind: KindDetect, Hint: "install it"})
	if r.Status != Missing {
		t.Errorf("KindDetect missing tool should be Missing; got %v", r.Status)
	}
}

func TestSummarize(t *testing.T) {
	got := Summarize([]Result{
		{Status: Installed}, {Status: Installed},
		{Status: Skipped}, {Status: Missing}, {Status: Failed},
	})
	for _, want := range []string{"2 installed", "1 already present", "1 need manual install", "1 failed"} {
		if !strings.Contains(got, want) {
			t.Errorf("Summarize missing %q; got %q", want, got)
		}
	}
}

func TestSafeJoinRejectsTraversal(t *testing.T) {
	base := t.TempDir()
	if _, err := safeJoin(base, "../escape"); err == nil {
		t.Error("safeJoin should reject path traversal")
	}
	if _, err := safeJoin(base, "ok/child.txt"); err != nil {
		t.Errorf("safeJoin should allow in-tree paths: %v", err)
	}
}

func TestStripComponents(t *testing.T) {
	if got := stripComponents("jdtls-1.0/plugins/a.jar", 1); got != "plugins/a.jar" {
		t.Errorf("stripComponents = %q", got)
	}
	if got := stripComponents("top", 1); got != "" {
		t.Errorf("stripComponents of shallow path = %q, want empty", got)
	}
	if got := stripComponents("/a/b", 0); got != "a/b" {
		t.Errorf("stripComponents strip 0 = %q", got)
	}
}

func indexOf(s []string, want string) int {
	for i, v := range s {
		if v == want {
			return i
		}
	}
	return -1
}
