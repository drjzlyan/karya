// Package project scaffolds new projects for the languages karya supports.
//
// The file generation is pure and deterministic: Scaffold writes a complete,
// self-contained project from embedded templates and never shells out to
// external tools (uv/cargo/go/npm), so scaffolding is reproducible, hermetic to
// test, and works offline. Best-effort side effects (git init) live in GitInit,
// separate from the pure logic, and the tmux "open in a session" step is handled
// by the CLI. Nothing here touches anything outside the target directory.
package project

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"
)

// Language is a supported project language.
type Language string

// Supported languages.
const (
	Python     Language = "python"
	Java       Language = "java"
	TypeScript Language = "typescript"
	Go         Language = "go"
	Cpp        Language = "cpp"
	Rust       Language = "rust"
)

// languageAliases maps user-facing aliases to canonical languages.
var languageAliases = map[string]Language{
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

// Languages lists the canonical language names for help/usage text.
var Languages = []string{
	string(Python), string(Java), string(TypeScript),
	string(Go), string(Cpp), string(Rust),
}

// ParseLanguage resolves a language name or alias to its canonical Language.
func ParseLanguage(s string) (Language, error) {
	if lang, ok := languageAliases[strings.ToLower(strings.TrimSpace(s))]; ok {
		return lang, nil
	}
	return "", fmt.Errorf("unknown language %q (supported: %s)", s, strings.Join(Languages, ", "))
}

// Spec is a validated, normalized scaffold request.
type Spec struct {
	Lang Language
	// Name is the name as given by the user. For go it is the module path
	// (github.com/user/app); for java it is the group id (com.example.app);
	// otherwise it is the project name.
	Name string
	// Basename is the last path component of Name, used for the created
	// directory and, for most languages, the project name.
	Basename string
}

// NewSpec validates the language and derives the Spec from a language name/alias
// and a project name.
func NewSpec(lang, name string) (Spec, error) {
	l, err := ParseLanguage(lang)
	if err != nil {
		return Spec{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Spec{}, fmt.Errorf("project name is required")
	}
	return Spec{Lang: l, Name: name, Basename: basename(name)}, nil
}

// basename returns the last component of a name split on '/' and '.'.
func basename(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool { return r == '/' || r == '.' })
	if len(parts) == 0 {
		return name
	}
	return parts[len(parts)-1]
}

// file is a generated file: a path relative to the project directory and its
// full contents.
type file struct {
	path    string
	content string
}

// Scaffold creates parentDir/<basename> and writes the project's files. It
// returns the absolute-or-parent-relative path of the created directory. It
// fails if the target directory already exists, leaving nothing behind.
func Scaffold(parentDir string, s Spec) (string, error) {
	dir := filepath.Join(parentDir, s.Basename)
	if _, err := os.Stat(dir); err == nil {
		return "", fmt.Errorf("%s already exists", dir)
	} else if !os.IsNotExist(err) {
		return "", err
	}

	files, err := render(s)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	for _, f := range files {
		full := filepath.Join(dir, f.path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(full, []byte(f.content), 0o644); err != nil {
			return "", err
		}
	}
	return dir, nil
}

// render produces the file set for a spec.
func render(s Spec) ([]file, error) {
	switch s.Lang {
	case Python:
		return renderPython(s), nil
	case Java:
		return renderJava(s), nil
	case TypeScript:
		return renderTypeScript(s), nil
	case Go:
		return renderGo(s), nil
	case Cpp:
		return renderCpp(s), nil
	case Rust:
		return renderRust(s), nil
	default:
		return nil, fmt.Errorf("unsupported language %q", s.Lang)
	}
}

// tmpl renders a static template with the given data. The templates are
// compile-time constants with no user-controlled delimiters, so parsing and
// execution cannot fail at runtime; a parse error is a programmer error.
func tmpl(name, text string, data any) string {
	var b strings.Builder
	t := template.Must(template.New(name).Parse(text))
	if err := t.Execute(&b, data); err != nil {
		panic(fmt.Sprintf("project: rendering %s: %v", name, err))
	}
	return b.String()
}

// className derives a valid Java class name from a project basename: it strips
// non-alphanumeric characters, ensures the result starts with a letter, and
// upper-cases the first letter. Empty input yields "App".
func className(base string) string {
	var b strings.Builder
	for _, r := range base {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	s := b.String()
	if s == "" {
		return "App"
	}
	if !unicode.IsLetter([]rune(s)[0]) {
		s = "App" + s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// GitInit runs `git init && git add -A && git commit` in dir, best-effort. It
// never fails the scaffold: a missing git binary or an empty commit is ignored.
// It returns an error only if git is unavailable, so callers can inform the user.
func GitInit(dir string) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found on PATH; skipped repository init")
	}
	run := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = dir
		_ = c.Run()
	}
	run("init")
	run("add", "-A")
	run("commit", "-m", "Initial commit")
	return nil
}

// ── Language templates ─────────────────────────────────────────────────────

func renderPython(s Spec) []file {
	n := s.Basename
	return []file{
		{"pyproject.toml", tmpl("pyproject", pyprojectTmpl, n)},
		{filepath.Join("src", n, "__init__.py"), ""},
		{filepath.Join("src", n, "main.py"), tmpl("pymain", pyMainTmpl, n)},
		{filepath.Join("tests", "test_main.py"), tmpl("pytest", pyTestTmpl, n)},
		{".gitignore", pythonGitignore},
	}
}

func renderJava(s Spec) []file {
	class := className(s.Basename)
	pkgPath := strings.ReplaceAll(s.Name, ".", "/")
	data := struct{ Name, GroupID, Class string }{s.Basename, s.Name, class}
	return []file{
		{"pom.xml", tmpl("pom", pomTmpl, data)},
		{filepath.Join("src/main/java", pkgPath, class+".java"), tmpl("javamain", javaMainTmpl, data)},
		{filepath.Join("src/test/java", pkgPath, class+"Test.java"), tmpl("javatest", javaTestTmpl, data)},
		{".gitignore", javaGitignore},
	}
}

func renderTypeScript(s Spec) []file {
	n := s.Basename
	return []file{
		{"package.json", tmpl("pkgjson", packageJSONTmpl, n)},
		{"tsconfig.json", tsconfig},
		{filepath.Join("src", "index.ts"), tmpl("tsindex", tsIndexTmpl, n)},
		{filepath.Join("test", "index.test.ts"), tsTest},
		{".gitignore", tsGitignore},
	}
}

func renderGo(s Spec) []file {
	data := struct{ Module, App string }{s.Name, s.Basename}
	return []file{
		{"go.mod", tmpl("gomod", goModTmpl, data)},
		{filepath.Join("cmd", s.Basename, "main.go"), tmpl("gomain", goMainTmpl, data)},
		{".gitignore", goGitignore},
	}
}

func renderCpp(s Spec) []file {
	n := s.Basename
	return []file{
		{"CMakeLists.txt", tmpl("cmake", cmakeTmpl, n)},
		{filepath.Join("src", "main.cpp"), tmpl("cppmain", cppMainTmpl, n)},
		{filepath.Join("tests", "test_main.cpp"), cppTest},
		{".gitignore", cppGitignore},
	}
}

func renderRust(s Spec) []file {
	n := s.Basename
	return []file{
		{"Cargo.toml", tmpl("cargo", cargoTmpl, n)},
		{filepath.Join("src", "main.rs"), tmpl("rustmain", rustMainTmpl, n)},
		{".gitignore", rustGitignore},
	}
}
