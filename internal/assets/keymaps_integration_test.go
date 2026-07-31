//go:build integration

package assets

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestKeymapConsistency is the guardrail for the unified keymap scheme: it drives
// headless Neovim over the embedded config (plugins stubbed, no network) and
// asserts every supported language exposes the identical <leader>c "Code"
// interface, and that close-buffer no longer lives on <leader>c. It fails loudly
// if a future change reintroduces per-language divergence.
//
// Tagged integration because it shells out to a real nvim binary.
func TestKeymapConsistency(t *testing.T) {
	if _, err := exec.LookPath("nvim"); err != nil {
		t.Skip("nvim not installed; skipping keymap guardrail")
	}
	nvimDir, err := filepath.Abs("nvim")
	if err != nil {
		t.Fatal(err)
	}
	harness, err := filepath.Abs(filepath.Join("testdata", "keymap_guard.lua"))
	if err != nil {
		t.Fatal(err)
	}

	// nvim -l writes print() to stderr; capture both streams. LSP "not found"
	// notices are interleaved but ignored by the field-matching parser below, and
	// completion is judged by the trailing OK marker rather than the exit code.
	out, _ := exec.Command("nvim", "-l", harness, nvimDir).CombinedOutput()

	// Every language must bind at least this identical core under <leader>c.
	core := []string{"c", "f", "h", "H", "l", "p", "r", "R", "t", "T"}
	langs := map[string]bool{"go": false, "rust": false, "typescript": false, "cpp": false, "python": false, "java": false}

	var sawOK bool
	globals := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		switch {
		case len(fields) >= 2 && fields[0] == "LANG":
			lang := fields[1]
			var suffixes string
			if len(fields) >= 3 {
				suffixes = fields[2]
			}
			set := map[string]bool{}
			for _, s := range strings.Split(suffixes, ",") {
				set[s] = true
			}
			for _, need := range core {
				if !set[need] {
					t.Errorf("language %q is missing <leader>c%s (has: %s)", lang, need, suffixes)
				}
			}
			langs[lang] = true
		case len(fields) == 3 && fields[0] == "GLOBAL":
			globals[fields[1]] = fields[2]
		case line == "OK":
			sawOK = true
		}
	}

	if !sawOK {
		t.Fatalf("keymap guard did not complete cleanly:\n%s", out)
	}
	for lang, seen := range langs {
		if !seen {
			t.Errorf("language %q was never reported by the keymap guard", lang)
		}
	}
	if globals["c"] != "absent" {
		t.Errorf("<leader>c must be the Code group prefix, not a global bind (got %q)", globals["c"])
	}
	if globals["x"] != "present" {
		t.Errorf("close-buffer should be bound to <leader>x (got %q)", globals["x"])
	}
}
