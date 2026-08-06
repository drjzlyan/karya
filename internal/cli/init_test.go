package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectToolchains(t *testing.T) {
	cases := []struct {
		name  string
		files []string
		want  []string // expected toolchain names, in order
	}{
		{"go", []string{"go.mod"}, []string{"Go"}},
		{"node", []string{"package.json"}, []string{"TypeScript/Node"}},
		{"rust", []string{"Cargo.toml"}, []string{"Rust"}},
		{"python pyproject", []string{"pyproject.toml"}, []string{"Python"}},
		{"python requirements", []string{"requirements.txt"}, []string{"Python"}},
		{"maven", []string{"pom.xml"}, []string{"Java (Maven)"}},
		{"gradle", []string{"build.gradle"}, []string{"Java (Gradle)"}},
		{"cmake", []string{"CMakeLists.txt"}, []string{"C++ (CMake)"}},
		{"dotnet", []string{"app.sln"}, []string{"C# (.NET)"}},
		{"multi", []string{"go.mod", "package.json"}, []string{"Go", "TypeScript/Node"}},
		{"none", nil, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			for _, f := range c.files {
				if err := os.WriteFile(filepath.Join(root, f), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got := detectToolchains(root)
			if len(got) != len(c.want) {
				t.Fatalf("detectToolchains = %v, want %v", got, c.want)
			}
			for i, name := range c.want {
				if got[i].Name != name {
					t.Errorf("toolchain %d = %q, want %q", i, got[i].Name, name)
				}
			}
		})
	}
}

func TestRenderAgentsMD(t *testing.T) {
	out := renderAgentsMD("proj", detectToolchains(mkRepo(t, "go.mod")))
	for _, want := range []string{"# AGENTS.md — proj", "go build ./...", "go test ./...", ".karya/tasks/<id>/SPEC.md", "karya help task"} {
		if !strings.Contains(out, want) {
			t.Errorf("AGENTS.md missing %q:\n%s", want, out)
		}
	}
}

func TestRenderAgentsMDNoToolchain(t *testing.T) {
	out := renderAgentsMD("proj", nil)
	if !strings.Contains(out, "No toolchain detected") {
		t.Errorf("want placeholder guidance:\n%s", out)
	}
}

// mkRepo creates a temp dir containing the given marker files.
func mkRepo(t *testing.T, files ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(root, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
