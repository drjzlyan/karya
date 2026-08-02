package toolreg

import (
	"strings"
	"testing"
)

func ids(tools []Tool) []string {
	out := make([]string, len(tools))
	for i, t := range tools {
		out[i] = t.ID
	}
	return out
}

func indexOf(s []string, want string) int {
	for i, v := range s {
		if v == want {
			return i
		}
	}
	return -1
}

func TestNewBuildsRegistry(t *testing.T) {
	r := New() // panics on duplicate IDs
	if len(r.All()) != len(registry) {
		t.Fatalf("All() = %d tools, want %d", len(r.All()), len(registry))
	}
	if _, ok := r.Get("gopls"); !ok {
		t.Error("expected gopls in registry")
	}
	if _, ok := r.Get("does-not-exist"); ok {
		t.Error("unexpected tool found")
	}
}

func TestByCategoryRuntimes(t *testing.T) {
	r := New()
	got := ids(r.ByCategory(Runtime))
	for _, want := range []string{"python-runtime", "node-runtime", "go-runtime", "java-runtime", "rust-runtime"} {
		if indexOf(got, want) == -1 {
			t.Errorf("Runtime category missing %q; got %v", want, got)
		}
	}
}

func TestPlanOrdersDependenciesFirst(t *testing.T) {
	r := New()
	plan, err := r.Plan([]string{"gopls"})
	if err != nil {
		t.Fatal(err)
	}
	got := ids(plan)
	goIdx, goplsIdx := indexOf(got, "go-runtime"), indexOf(got, "gopls")
	if goIdx == -1 || goplsIdx == -1 {
		t.Fatalf("plan missing runtime or tool: %v", got)
	}
	if goIdx > goplsIdx {
		t.Errorf("dependency go-runtime must precede gopls; got %v", got)
	}
}

func TestPlanDedupesSharedDependency(t *testing.T) {
	r := New()
	plan, err := r.Plan([]string{"gopls", "goimports", "delve"})
	if err != nil {
		t.Fatal(err)
	}
	got := ids(plan)
	count := 0
	for _, id := range got {
		if id == "go-runtime" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("shared dependency go-runtime should appear once; got %d in %v", count, got)
	}
}

func TestPlanUnknownToolErrors(t *testing.T) {
	r := New()
	if _, err := r.Plan([]string{"cobol-server"}); err == nil {
		t.Error("expected error for unknown tool id")
	}
}

func TestPlanDetectsCycle(t *testing.T) {
	// Build a small cyclic registry directly (the shipped catalog is acyclic).
	r := &Registry{byID: map[string]Tool{
		"a": {ID: "a", Dependencies: []string{"b"}},
		"b": {ID: "b", Dependencies: []string{"a"}},
	}}
	if _, err := r.Plan([]string{"a"}); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Errorf("expected cycle error, got %v", err)
	}
}

func TestPlanEmpty(t *testing.T) {
	r := New()
	plan, err := r.Plan(nil)
	if err != nil || len(plan) != 0 {
		t.Errorf("Plan(nil) = %v, %v; want empty, nil", ids(plan), err)
	}
}

func TestEssentialIDs(t *testing.T) {
	r := New()
	got := r.EssentialIDs()
	want := map[string]bool{"tmux": true, "neovim": true}
	if len(got) != len(want) {
		t.Fatalf("EssentialIDs() = %v, want tmux+neovim", got)
	}
	for _, id := range got {
		if !want[id] {
			t.Errorf("unexpected essential id %q", id)
		}
	}
}
