package toolreg

import "testing"

// TestCatalogWellFormed guards the hand-written catalog data: stable IDs, a
// display name and category on every tool, a way to detect each tool (executable
// or artifact), install hints for detect-only tools, and dependency IDs that
// actually exist in the registry.
func TestCatalogWellFormed(t *testing.T) {
	r := New()
	for _, tool := range r.All() {
		if tool.ID == "" {
			t.Errorf("tool %q has empty ID", tool.Name)
		}
		if tool.Name == "" {
			t.Errorf("tool %q has empty Name", tool.ID)
		}
		if tool.Category == "" {
			t.Errorf("tool %q has empty Category", tool.ID)
		}
		if tool.Method == "" {
			t.Errorf("tool %q has empty Method", tool.ID)
		}
		if tool.Executable == "" && tool.Artifact == "" {
			t.Errorf("tool %q has neither Executable nor Artifact", tool.ID)
		}
		if tool.Method == MethodDetect && tool.Hint == "" {
			t.Errorf("detect-only tool %q must carry an install Hint", tool.ID)
		}
		for _, dep := range tool.Dependencies {
			if _, ok := r.Get(dep); !ok {
				t.Errorf("tool %q depends on unknown tool %q", tool.ID, dep)
			}
		}
	}
}

// TestCatalogPrefersMise encodes the install-order guideline: language runtimes
// and core infra must be provisioned by the single vendored mise, which is what
// keeps karya's isolation guarantee cheap and consistent. Per-language LSP tools
// legitimately use language-native installers (uv/npm/go/rustup); this test only
// pins the runtimes/infra that mise is meant to own.
func TestCatalogPrefersMise(t *testing.T) {
	r := New()
	for _, tool := range r.ByCategory(Runtime) {
		if tool.Method != MethodMise {
			t.Errorf("runtime %q should install via mise, got %q", tool.ID, tool.Method)
		}
	}
	for _, id := range []string{"tmux", "neovim", "uv"} {
		tool, ok := r.Get(id)
		if !ok {
			t.Fatalf("core infra tool %q missing from catalog", id)
		}
		if tool.Method != MethodMise {
			t.Errorf("core infra %q should install via mise, got %q", id, tool.Method)
		}
	}
}

// TestEssentialToolsPresent confirms the launch-critical tools are marked
// essential so the bootstrap treats a failure to provide them as a hard error.
func TestEssentialToolsPresent(t *testing.T) {
	r := New()
	for _, id := range []string{"tmux", "neovim"} {
		tool, ok := r.Get(id)
		if !ok || !tool.Essential {
			t.Errorf("tool %q must exist and be Essential (ok=%v)", id, ok)
		}
	}
}
