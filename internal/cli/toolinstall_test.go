package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/drjzlyan/karya/internal/config"
	"github.com/drjzlyan/karya/internal/toolreg"
)

// testApp builds an app wired to a resolver over a temp karya prefix, enough to
// exercise the registry-driven launch bootstrap without newApp's real setup.
func testApp(t *testing.T) *app {
	t.Helper()
	p := config.Paths{Data: t.TempDir(), Config: t.TempDir(), State: t.TempDir(), Cache: t.TempDir()}
	reg := toolreg.New()
	return &app{paths: p, reg: reg, resolver: toolreg.NewResolver(p, reg)}
}

func seedBin(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureCoreFastPathNoInstall(t *testing.T) {
	a := testApp(t)
	// Seed the essential executables into the managed tool bin so they resolve;
	// ensureCore must then be a no-op (never touching mise).
	for _, exe := range []string{"tmux", "nvim"} {
		seedBin(t, a.paths.ToolsBin(), exe)
	}
	if !a.allResolve(a.reg.EssentialIDs()) {
		t.Fatal("seeded essentials should resolve")
	}
	if err := a.ensureCore(); err != nil {
		t.Errorf("ensureCore with essentials present should be a no-op; got %v", err)
	}
}

func TestAllResolveDetectsMissing(t *testing.T) {
	a := testApp(t)
	// Clear PATH so system tmux/nvim don't satisfy the check, and seed neither.
	t.Setenv("PATH", "")
	if a.allResolve(a.reg.EssentialIDs()) {
		t.Error("allResolve should be false when essentials are absent")
	}
}
