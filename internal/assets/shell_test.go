package assets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExtractShellInit checks karya's shell startup files land with the expected
// names and that they source the user's own rc before wiring starship — the
// isolation contract that karya never edits the user's ~/.zshrc / ~/.bashrc.
func TestExtractShellInit(t *testing.T) {
	dir := t.TempDir()
	if err := ExtractShellInit(dir); err != nil {
		t.Fatalf("ExtractShellInit: %v", err)
	}

	cases := []struct {
		name     string
		contains []string
	}{
		{".zshrc", []string{`source "${HOME}/.zshrc"`, "starship init zsh"}},
		{"bashrc", []string{`source "${HOME}/.bashrc"`, "starship init bash"}},
		{"starship.toml", []string{"format ="}},
	}
	for _, c := range cases {
		b, err := os.ReadFile(filepath.Join(dir, c.name))
		if err != nil {
			t.Errorf("read %s: %v", c.name, err)
			continue
		}
		for _, want := range c.contains {
			if !strings.Contains(string(b), want) {
				t.Errorf("%s missing %q", c.name, want)
			}
		}
	}

	// Idempotent: a second extraction over the same dir succeeds (refresh path).
	if err := ExtractShellInit(dir); err != nil {
		t.Fatalf("second ExtractShellInit: %v", err)
	}
}
