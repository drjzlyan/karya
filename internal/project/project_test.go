package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLanguage(t *testing.T) {
	cases := map[string]Language{
		"python":     Python,
		"java":       Java,
		"typescript": TypeScript,
		"ts":         TypeScript,
		"node":       TypeScript,
		"go":         Go,
		"golang":     Go,
		"cpp":        Cpp,
		"c":          Cpp,
		"c++":        Cpp,
		"rust":       Rust,
		"rs":         Rust,
	}
	for in, want := range cases {
		got, err := ParseLanguage(in)
		if err != nil {
			t.Errorf("ParseLanguage(%q) unexpected error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseLanguage(%q) = %q, want %q", in, got, want)
		}
	}
	if _, err := ParseLanguage("brainfuck"); err == nil {
		t.Error("ParseLanguage(\"brainfuck\") = nil error, want error")
	}
}

func TestNewSpecBasename(t *testing.T) {
	cases := []struct {
		lang, name   string
		wantBasename string
	}{
		{"python", "myapp", "myapp"},
		{"java", "com.example.myapp", "myapp"},
		{"go", "github.com/user/myapp", "myapp"},
		{"typescript", "myapp", "myapp"},
		{"rust", "my-lib", "my-lib"},
	}
	for _, c := range cases {
		s, err := NewSpec(c.lang, c.name)
		if err != nil {
			t.Fatalf("NewSpec(%q,%q): %v", c.lang, c.name, err)
		}
		if s.Basename != c.wantBasename {
			t.Errorf("NewSpec(%q,%q).Basename = %q, want %q", c.lang, c.name, s.Basename, c.wantBasename)
		}
		if s.Name != c.name {
			t.Errorf("NewSpec(%q,%q).Name = %q, want %q", c.lang, c.name, s.Name, c.name)
		}
	}
}

func TestNewSpecEmpty(t *testing.T) {
	if _, err := NewSpec("go", ""); err == nil {
		t.Error("NewSpec with empty name = nil error, want error")
	}
}

// wantFiles maps each language to relative paths that must exist and a
// substring the primary source file must contain.
func TestScaffold(t *testing.T) {
	cases := []struct {
		lang, name string
		dirName    string   // expected created directory (basename)
		files      []string // relative paths that must exist
		greetFile  string   // file expected to contain the greeting
		greet      string   // greeting substring
	}{
		{
			lang: "python", name: "myapp", dirName: "myapp",
			files:     []string{"pyproject.toml", "src/myapp/__init__.py", "src/myapp/main.py", "tests/test_main.py", ".gitignore"},
			greetFile: "src/myapp/main.py", greet: "Hello from myapp!",
		},
		{
			lang: "java", name: "com.example.myapp", dirName: "myapp",
			files:     []string{"pom.xml", "src/main/java/com/example/myapp/Myapp.java", "src/test/java/com/example/myapp/MyappTest.java", ".gitignore"},
			greetFile: "src/main/java/com/example/myapp/Myapp.java", greet: "Hello from myapp!",
		},
		{
			lang: "typescript", name: "myapp", dirName: "myapp",
			files:     []string{"package.json", "tsconfig.json", "src/index.ts", "test/index.test.ts", ".gitignore"},
			greetFile: "src/index.ts", greet: "Hello from myapp!",
		},
		{
			lang: "go", name: "github.com/user/myapp", dirName: "myapp",
			files:     []string{"go.mod", "cmd/myapp/main.go", ".gitignore"},
			greetFile: "cmd/myapp/main.go", greet: "Hello from github.com/user/myapp!",
		},
		{
			lang: "cpp", name: "myapp", dirName: "myapp",
			files:     []string{"CMakeLists.txt", "src/main.cpp", "tests/test_main.cpp", ".gitignore"},
			greetFile: "src/main.cpp", greet: "Hello from myapp!",
		},
		{
			lang: "rust", name: "myapp", dirName: "myapp",
			files:     []string{"Cargo.toml", "src/main.rs", ".gitignore"},
			greetFile: "src/main.rs", greet: "Hello from myapp!",
		},
	}

	for _, c := range cases {
		t.Run(c.lang, func(t *testing.T) {
			parent := t.TempDir()
			s, err := NewSpec(c.lang, c.name)
			if err != nil {
				t.Fatalf("NewSpec: %v", err)
			}
			dir, err := Scaffold(parent, s)
			if err != nil {
				t.Fatalf("Scaffold: %v", err)
			}
			if got := filepath.Base(dir); got != c.dirName {
				t.Errorf("created dir = %q, want %q", got, c.dirName)
			}
			for _, rel := range c.files {
				if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
					t.Errorf("missing expected file %q: %v", rel, err)
				}
			}
			b, err := os.ReadFile(filepath.Join(dir, c.greetFile))
			if err != nil {
				t.Fatalf("read %q: %v", c.greetFile, err)
			}
			if !strings.Contains(string(b), c.greet) {
				t.Errorf("%q does not contain greeting %q", c.greetFile, c.greet)
			}
		})
	}
}

func TestScaffoldGoModule(t *testing.T) {
	s, err := NewSpec("go", "github.com/user/myapp")
	if err != nil {
		t.Fatal(err)
	}
	dir, err := Scaffold(t.TempDir(), s)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "module github.com/user/myapp") {
		t.Errorf("go.mod missing module line, got:\n%s", b)
	}
}

func TestScaffoldExistingDirFails(t *testing.T) {
	parent := t.TempDir()
	if err := os.Mkdir(filepath.Join(parent, "myapp"), 0o755); err != nil {
		t.Fatal(err)
	}
	s, _ := NewSpec("python", "myapp")
	if _, err := Scaffold(parent, s); err == nil {
		t.Error("Scaffold into existing dir = nil error, want error")
	}
}

func TestClassName(t *testing.T) {
	cases := map[string]string{
		"myapp":  "Myapp",
		"my-app": "Myapp",
		"MyApp":  "MyApp",
		"123":    "App123", // leading non-letter can't start an identifier
		"":       "App",
	}
	for in, want := range cases {
		if got := className(in); got != want {
			t.Errorf("className(%q) = %q, want %q", in, got, want)
		}
	}
}
