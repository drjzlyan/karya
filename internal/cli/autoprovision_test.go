package cli

import (
	"slices"
	"testing"

	"github.com/drjzlyan/karya/internal/toolreg"
)

func TestLanguageToolIDs(t *testing.T) {
	reg := toolreg.New()

	// Go should include its language server and formatter.
	got := languageToolIDs(reg, "go")
	for _, want := range []string{"gopls", "goimports"} {
		if !slices.Contains(got, want) {
			t.Fatalf("go tools %v missing %q", got, want)
		}
	}
	// It must NOT include runtimes/debuggers directly (those come via deps).
	if slices.Contains(got, "go-runtime") {
		t.Fatalf("languageToolIDs should not list the runtime directly: %v", got)
	}

	// A language with no per-language catalog tooling yields nothing.
	if ids := languageToolIDs(reg, "cobol"); len(ids) != 0 {
		t.Fatalf("unknown language should yield no tools, got %v", ids)
	}

	// Every returned tool is an LSP/formatter/linter located at that language.
	for _, lang := range []string{"python", "typescript", "rust"} {
		for _, id := range languageToolIDs(reg, lang) {
			tool, ok := reg.Get(id)
			if !ok {
				t.Fatalf("%s: unknown tool id %q", lang, id)
			}
			if tool.Location.Kind != toolreg.LocLang || tool.Location.Lang != lang {
				t.Fatalf("%s: tool %q not located at language", lang, id)
			}
			switch tool.Category {
			case toolreg.LanguageServer, toolreg.Formatter, toolreg.Linter:
			default:
				t.Fatalf("%s: tool %q has unexpected category %v", lang, id, tool.Category)
			}
		}
	}
}
