package lang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeManagerEnsureWritesConfig(t *testing.T) {
	// Force mise to appear absent so Ensure is deterministic (no install runs).
	t.Setenv("PATH", "")

	dir := t.TempDir()
	cfg := filepath.Join(dir, "mise", "config.toml")
	sel := NewSelection()
	sel.Set("go", []string{"1.26"})

	rm := RuntimeManager{
		MiseConfigPath: cfg,
		GoPath:         filepath.Join(dir, "go"),
		CargoHome:      filepath.Join(dir, "cargo"),
	}
	ran, err := rm.Ensure(sel, []MiseTool{{Key: "taplo"}})
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if ran {
		t.Error("Ensure should report ran=false when mise is not installed")
	}

	data, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatalf("config not written: %v", err)
	}
	got := string(data)
	for _, want := range []string{"[tools]", "taplo", "go = [\"1.26\"]"} {
		if !strings.Contains(got, want) {
			t.Errorf("generated config missing %q; got:\n%s", want, got)
		}
	}
}
