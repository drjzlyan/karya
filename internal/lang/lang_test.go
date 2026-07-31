package lang

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDedupVersions(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		mode DedupMode
		want []string
	}{
		{
			name: "minor keeps one per major.minor, ascending, first patch wins",
			in:   []string{"3.13.1", "3.13.5", "3.14.0", "3.14.2"},
			mode: DedupMinor,
			want: []string{"3.13", "3.14"},
		},
		{
			name: "major keeps one per major",
			in:   []string{"20.1.0", "20.9.0", "21.0.2", "22.0.0"},
			mode: DedupMajor,
			want: []string{"20", "21", "22"},
		},
		{
			name: "minor on single-component versions",
			in:   []string{"24", "22", "24"},
			mode: DedupMinor,
			want: []string{"24", "22"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DedupVersions(tt.in, tt.mode); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DedupVersions() = %v, want %v", got, tt.want)
			}
		})
	}
}

// fakeLister returns canned remote versions per tool.
type fakeLister struct {
	byTool map[string][]string
	err    error
}

func (f fakeLister) ListRemote(tool string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byTool[tool], nil
}

func TestAvailableVersionsFiltersAndDedups(t *testing.T) {
	l, _ := Find("python")
	lister := fakeLister{byTool: map[string][]string{
		"python": {"3.12.0", "3.13.0", "3.13.4", "3.14.0", "3.15.0a1", "3.14.1-dev"},
	}}
	got := AvailableVersions(lister, l)
	want := []string{"3.12", "3.13", "3.14"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("AvailableVersions() = %v, want %v", got, want)
	}
}

func TestAvailableVersionsFallsBackOffline(t *testing.T) {
	l, _ := Find("go")
	// Empty remote (mise absent) → single fallback version.
	got := AvailableVersions(fakeLister{}, l)
	if !reflect.DeepEqual(got, []string{l.Fallback}) {
		t.Errorf("offline AvailableVersions() = %v, want [%s]", got, l.Fallback)
	}
}

func TestAvailableVersionsSystemLanguage(t *testing.T) {
	l, _ := Find("cpp")
	got := AvailableVersions(fakeLister{byTool: map[string][]string{}}, l)
	if !reflect.DeepEqual(got, []string{"system"}) {
		t.Errorf("cpp AvailableVersions() = %v, want [system]", got)
	}
}

func TestJavaVersionsPrefersPlainThenDistros(t *testing.T) {
	l, _ := Find("java")
	// 17 plain, 21 only as temurin, 25 plain; 11 only corretto; noise filtered.
	lister := fakeLister{byTool: map[string][]string{
		"java": {
			"8.0.1", "corretto-11.0.2",
			"17.0.9", "temurin-21.0.1", "25.0.0", "25.0.1-ea",
		},
	}}
	got := AvailableVersions(lister, l)
	// majors 8..25: 8 has plain -> "8"; 11 only corretto -> "corretto-11";
	// 17 plain -> "17"; 21 temurin -> "temurin-21"; 25 plain -> "25".
	want := []string{"8", "corretto-11", "17", "temurin-21", "25"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("java AvailableVersions() = %v, want %v", got, want)
	}
}

func TestFindAliases(t *testing.T) {
	for _, in := range []string{"ts", "TypeScript", "node", "  js "} {
		l, ok := Find(in)
		if !ok || l.Name != "typescript" {
			t.Errorf("Find(%q) = %+v, %v; want typescript", in, l, ok)
		}
	}
	if _, ok := Find("cobol"); ok {
		t.Error("Find(cobol) should not resolve")
	}
}

func TestSelectionSetRemoveOrder(t *testing.T) {
	s := NewSelection()
	s.Set("python", []string{"3.14", "3.13"})
	s.Set("go", []string{"1.26"})
	s.Set("java", []string{"25"})
	// Re-setting python keeps its original position and replaces versions.
	s.Set("python", []string{"3.14"})
	if got := s.Langs(); !reflect.DeepEqual(got, []string{"python", "go", "java"}) {
		t.Errorf("order = %v", got)
	}
	if got := s.Versions("python"); !reflect.DeepEqual(got, []string{"3.14"}) {
		t.Errorf("python versions = %v", got)
	}
	if s.Primary("python") != "3.14" {
		t.Errorf("primary = %q", s.Primary("python"))
	}
	s.Remove("go")
	if s.Has("go") {
		t.Error("go should be removed")
	}
	if got := s.Langs(); !reflect.DeepEqual(got, []string{"python", "java"}) {
		t.Errorf("order after remove = %v", got)
	}
	// Setting empty versions removes the language.
	s.Set("java", []string{"", "  "})
	if s.Has("java") {
		t.Error("java should be removed by empty set")
	}
}

func TestSelectionRoundTrip(t *testing.T) {
	s := NewSelection()
	s.Set("python", []string{"3.14", "3.13"})
	s.Set("cpp", []string{"system"})

	path := filepath.Join(t.TempDir(), "languages.local")
	if err := SaveSelection(path, s); err != nil {
		t.Fatalf("SaveSelection: %v", err)
	}
	got, err := LoadSelection(path)
	if err != nil {
		t.Fatalf("LoadSelection: %v", err)
	}
	if !reflect.DeepEqual(got.Langs(), []string{"python", "cpp"}) {
		t.Errorf("langs = %v", got.Langs())
	}
	if !reflect.DeepEqual(got.Versions("python"), []string{"3.14", "3.13"}) {
		t.Errorf("python versions = %v", got.Versions("python"))
	}
}

func TestLoadSelectionMissingFile(t *testing.T) {
	s, err := LoadSelection(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("LoadSelection missing: %v", err)
	}
	if !s.Empty() {
		t.Error("missing file should yield empty selection")
	}
}

func TestParseSelectionIgnoresCommentsAndBlanks(t *testing.T) {
	s := ParseSelection("# header\n\npython=3.14,3.13\n  \njava = 25 \n")
	if !reflect.DeepEqual(s.Langs(), []string{"python", "java"}) {
		t.Errorf("langs = %v", s.Langs())
	}
	if s.Primary("java") != "25" {
		t.Errorf("java primary = %q", s.Primary("java"))
	}
}

func TestGenerateMiseConfig(t *testing.T) {
	s := NewSelection()
	s.Set("python", []string{"3.14", "3.13"})
	s.Set("typescript", []string{"24"})
	s.Set("go", []string{"1.26"})
	s.Set("cpp", []string{"system"})
	s.Set("java", []string{"25"})
	s.Set("rust", []string{"1.97"})

	always := []MiseTool{{Key: "taplo"}, {Key: "marksman", Version: "1.2.3"}}
	out := GenerateMiseConfig(s, MiseVars{GoPath: "/k/go", CargoHome: "/k/cargo"}, always)

	for _, want := range []string{
		`taplo = "latest"`,   // always-on tool, empty version → latest
		`marksman = "1.2.3"`, // always-on tool with an explicit version
		`python = ["3.14", "3.13"]`,
		`node = ["24"]`, // typescript maps to node
		`go = ["1.26"]`,
		`java = ["25"]`,
		`rust = ["1.97"]`,
		`JAVA_HOME = { value = "{{ exec(command='mise where java') }}", tools = true }`,
		`GOPATH = "/k/go"`,
		`CARGO_HOME = "/k/cargo"`,
		"experimental = true",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("mise config missing %q; got:\n%s", want, out)
		}
	}
	// C/C++ has no managed runtime — it must not appear in [tools].
	if strings.Contains(out, "cpp") || strings.Contains(out, "system") {
		t.Errorf("cpp/system should be absent from mise config; got:\n%s", out)
	}
}

func TestWriteMiseConfig(t *testing.T) {
	s := NewSelection()
	s.Set("go", []string{"1.26"})
	path := filepath.Join(t.TempDir(), "mise", "config.toml")
	if err := WriteMiseConfig(path, s, MiseVars{GoPath: "/k/go"}, []MiseTool{{Key: "taplo"}}); err != nil {
		t.Fatalf("WriteMiseConfig: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(b), `go = ["1.26"]`) {
		t.Errorf("written config missing go tools; got:\n%s", b)
	}
	if !strings.Contains(string(b), `taplo = "latest"`) {
		t.Errorf("written config missing always-on tool; got:\n%s", b)
	}
}
