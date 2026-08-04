package toolreg

import "testing"

// TestCoreAndDocToolsPresent guards the expanded catalog: the core CLI and
// documentation tools karya manages so nothing is assumed to exist globally.
func TestCoreAndDocToolsPresent(t *testing.T) {
	r := New()
	// Core CLI utilities, provisioned via mise (git is the detect exception).
	for _, id := range []string{
		"jq", "yq", "fd", "ripgrep", "fzf", "lazygit", "starship", "bat", "eza",
		"delta", "tree", "zoxide", "just", "watchexec", "hyperfine", "shellcheck",
		"shfmt", "gh",
	} {
		tool, ok := r.Get(id)
		if !ok {
			t.Errorf("core CLI tool %q missing from catalog", id)
			continue
		}
		if tool.Method != MethodMise {
			t.Errorf("core CLI tool %q should install via mise, got %q", id, tool.Method)
		}
		if tool.Location.Kind != LocCore {
			t.Errorf("core CLI tool %q should be LocCore, got %q", id, tool.Location.Kind)
		}
	}

	git, ok := r.Get("git")
	if !ok || git.Method != MethodDetect {
		t.Errorf("git should be a detect-only tool; got ok=%v method=%q", ok, git.Method)
	}

	// Documentation tools land under the docs category.
	for _, id := range []string{"markdownlint", "yamllint", "yamlfmt", "jsonlint"} {
		tool, ok := r.Get(id)
		if !ok {
			t.Errorf("doc tool %q missing from catalog", id)
			continue
		}
		if tool.Location.Kind != LocDocs {
			t.Errorf("doc tool %q should be LocDocs, got %q", id, tool.Location.Kind)
		}
	}
}
