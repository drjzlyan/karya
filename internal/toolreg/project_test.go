package toolreg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectProjectFindsNearestConfig(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".tool-versions"), []byte("python 3.11\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	pe, ok := DetectProject(sub)
	if !ok {
		t.Fatal("expected to detect project from a nested dir")
	}
	if pe.Root != root {
		t.Errorf("Root = %q, want %q", pe.Root, root)
	}
	if len(pe.Configs) != 1 || filepath.Base(pe.Configs[0]) != ".tool-versions" {
		t.Errorf("Configs = %v, want one .tool-versions", pe.Configs)
	}
}

func TestDetectProjectPrefersNearest(t *testing.T) {
	outer := t.TempDir()
	inner := filepath.Join(outer, "inner")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{outer, inner} {
		if err := os.WriteFile(filepath.Join(dir, "mise.toml"), []byte("[tools]\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pe, ok := DetectProject(inner)
	if !ok || pe.Root != inner {
		t.Errorf("DetectProject should prefer the nearest config dir; got %+v ok=%v", pe, ok)
	}
}

func TestDetectProjectNoConfig(t *testing.T) {
	if _, ok := DetectProject(t.TempDir()); ok {
		t.Error("a dir with no version config should not be a project")
	}
}

func TestResolverPrefersProjectRuntime(t *testing.T) {
	rv, bin := testResolver(t, nil)
	seed(t, bin, "python") // a managed python shim also exists
	rv = rv.WithProject(&ProjectEnv{Root: "/proj"})
	rv.projectWhich = func(root, exe string) (string, bool) {
		if root == "/proj" && exe == "python" {
			return "/proj/.mise/python", true
		}
		return "", false
	}
	got, ok := rv.Resolve("python-runtime")
	if !ok || got.Source != SourceProject || got.Path != "/proj/.mise/python" {
		t.Fatalf("Resolve(python-runtime) = %+v ok=%v; want project", got, ok)
	}
}

func TestResolverProjectFallsBackToManaged(t *testing.T) {
	rv, bin := testResolver(t, nil)
	want := seed(t, bin, "python")
	rv = rv.WithProject(&ProjectEnv{Root: "/proj"})
	rv.projectWhich = func(string, string) (string, bool) { return "", false } // not pinned
	got, ok := rv.Resolve("python-runtime")
	if !ok || got.Source != SourceManaged || got.Path != want {
		t.Fatalf("Resolve(python-runtime) = %+v ok=%v; want managed fallback", got, ok)
	}
}
