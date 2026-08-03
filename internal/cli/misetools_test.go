package cli

import (
	"testing"

	"github.com/drjzlyan/karya/internal/lang"
	"github.com/drjzlyan/karya/internal/toolreg"
)

func TestMiseToolsForDeclaresBaselineAndSelectedLangs(t *testing.T) {
	reg := toolreg.New()
	sel := lang.NewSelection()
	sel.Set("java", []string{"25"})

	keys := map[string]bool{}
	for _, mt := range miseToolsFor(reg, sel) {
		keys[mt.Key] = true
	}

	// Baseline mise tools (core CLI, docs, always-on servers, infra) are always
	// declared so their shims resolve.
	for _, want := range []string{"jq", "ripgrep", "taplo", "marksman", "yamlfmt", "tmux", "neovim", "uv"} {
		if !keys[want] {
			t.Errorf("baseline mise tool %q not declared", want)
		}
	}
	// Selected language's mise tools are declared.
	for _, want := range []string{"maven", "gradle", "google-java-format"} {
		if !keys[want] {
			t.Errorf("selected java mise tool %q not declared", want)
		}
	}
	// Unselected language's mise tools are NOT declared.
	if keys["rust-analyzer"] || keys["cmake"] {
		t.Error("unselected-language mise tools should not be declared")
	}
	// Runtimes are declared from the selection's [tools], not here.
	if keys["python"] || keys["node"] || keys["go"] {
		t.Error("runtimes should not be in the mise-tools list")
	}
}
