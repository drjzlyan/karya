package toolreg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/drjzlyan/karya/internal/config"
)

// testResolver builds a resolver over a temp karya prefix with a controllable
// PATH lookup, and returns the tool-bin dir for seeding managed binaries.
func testResolver(t *testing.T, onPath map[string]string) (*Resolver, string) {
	t.Helper()
	data := t.TempDir()
	p := config.Paths{Data: data, Config: t.TempDir(), State: t.TempDir(), Cache: t.TempDir()}
	rv := NewResolver(p, New())
	rv.lookPath = func(name string) (string, error) {
		if path, ok := onPath[name]; ok {
			return path, nil
		}
		return "", os.ErrNotExist
	}
	bin := p.ToolsBin()
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	return rv, bin
}

func seed(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResolvePrefersManagedBin(t *testing.T) {
	rv, bin := testResolver(t, map[string]string{"gopls": "/usr/bin/gopls"})
	want := seed(t, bin, "gopls")
	got, ok := rv.Resolve("gopls")
	if !ok || got.Path != want || got.Source != SourceManaged {
		t.Fatalf("Resolve(gopls) = %+v, ok=%v; want managed %s", got, ok, want)
	}
}

func TestResolveFallsBackToSystem(t *testing.T) {
	rv, _ := testResolver(t, map[string]string{"gopls": "/usr/bin/gopls"})
	got, ok := rv.Resolve("gopls")
	if !ok || got.Source != SourceSystem || got.Path != "/usr/bin/gopls" {
		t.Fatalf("Resolve(gopls) = %+v, ok=%v; want system", got, ok)
	}
}

func TestResolveArtifactOnlyTool(t *testing.T) {
	rv, _ := testResolver(t, nil)
	want := seed(t, rv.paths.ToolsDir(), "lombok.jar")
	got, ok := rv.Resolve("lombok")
	if !ok || got.Path != want || got.Source != SourceManaged {
		t.Fatalf("Resolve(lombok) = %+v, ok=%v; want managed artifact %s", got, ok, want)
	}
}

func TestResolveBareExecutableName(t *testing.T) {
	// An id not in the registry is treated as a bare command name.
	rv, bin := testResolver(t, nil)
	want := seed(t, bin, "jq")
	if got := rv.Path("jq"); got != want {
		t.Errorf("Path(jq) = %q, want %q", got, want)
	}
}

func TestResolveMissing(t *testing.T) {
	rv, _ := testResolver(t, nil)
	if got, ok := rv.Resolve("gopls"); ok || got.Source != SourceMissing {
		t.Errorf("Resolve(missing) = %+v, ok=%v; want missing", got, ok)
	}
	if rv.Path("gopls") != "" {
		t.Error("Path of missing tool should be empty")
	}
}

func TestManifestOmitsMissingTools(t *testing.T) {
	rv, bin := testResolver(t, nil)
	seed(t, bin, "gopls")
	m := rv.Manifest()
	if e, ok := m.Tools["gopls"]; !ok || e.Source != "managed" {
		t.Errorf("manifest should include managed gopls; got %+v", m.Tools["gopls"])
	}
	if _, ok := m.Tools["ruff"]; ok {
		t.Error("manifest should omit unresolved ruff")
	}
}

func TestWriteManifestRoundTrips(t *testing.T) {
	rv, bin := testResolver(t, nil)
	seed(t, bin, "gopls")
	path := rv.paths.ToolsManifest()
	if err := WriteManifest(path, rv.Manifest()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
}
