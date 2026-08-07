package finder

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/drjzlyan/karya/internal/cellbuf"
	"github.com/drjzlyan/karya/internal/term"
)

func TestMatchSubsequence(t *testing.T) {
	if _, ok := Match("fb", "foo/bar.go"); !ok {
		t.Fatal("fb should match foo/bar.go")
	}
	if _, ok := Match("xyz", "foo/bar.go"); ok {
		t.Fatal("xyz should not match")
	}
	if _, ok := Match("", "anything"); !ok {
		t.Fatal("empty query matches everything")
	}
}

func TestFilterRanksSegmentAndShorter(t *testing.T) {
	items := []string{"internal/remaining.go", "main.go", "cmd/main.go", "readme"}
	got := Filter("main", items)
	// Only the three containing the subsequence "main"; "readme" excluded.
	if slices.Contains(got, "readme") {
		t.Fatalf("readme should not match 'main': %v", got)
	}
	// main.go (segment start + shortest) should rank first.
	if got[0] != "main.go" {
		t.Fatalf("expected main.go first, got %v", got)
	}
}

func TestFilterEmptyReturnsAll(t *testing.T) {
	items := []string{"a", "b"}
	if got := Filter("  ", items); len(got) != 2 {
		t.Fatalf("empty query should return all, got %v", got)
	}
}

func TestWalkFilesSkipsVCSDirs(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "src"), 0o755)
	os.MkdirAll(filepath.Join(root, ".git"), 0o755)
	os.WriteFile(filepath.Join(root, "src", "a.go"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(root, "README.md"), []byte("y"), 0o644)
	os.WriteFile(filepath.Join(root, ".git", "config"), []byte("z"), 0o644)

	files := walkFiles(root)
	if !slices.Contains(files, "src/a.go") || !slices.Contains(files, "README.md") {
		t.Fatalf("expected project files, got %v", files)
	}
	for _, f := range files {
		if strings.HasPrefix(f, ".git/") {
			t.Fatalf(".git should be skipped, got %v", files)
		}
	}
}

func TestFinderViewFilterAndOpen(t *testing.T) {
	f := New([]string{"main.go", "cmd/root.go", "internal/x.go"})
	// Type "root" → narrows to cmd/root.go.
	for _, r := range "root" {
		f.HandleKey(term.RuneKey(r))
	}
	if len(f.filtered) != 1 || f.filtered[0] != "cmd/root.go" {
		t.Fatalf("filter wrong: %v", f.filtered)
	}
	f.HandleKey(term.Named(term.SymEnter))
	if got := f.OpenRequest(); got != "cmd/root.go" {
		t.Fatalf("open request = %q", got)
	}
	if got := f.OpenRequest(); got != "" {
		t.Fatalf("open request should be consumed, got %q", got)
	}
}

func TestFinderNavAndClose(t *testing.T) {
	f := New([]string{"a", "b", "c"})
	f.HandleKey(term.Named(term.SymDown))
	f.HandleKey(term.Named(term.SymDown))
	if f.sel != 2 {
		t.Fatalf("sel = %d want 2", f.sel)
	}
	f.HandleKey(term.Named(term.SymEsc))
	if !f.Done() {
		t.Fatal("esc should close")
	}
}

func TestFinderBackspace(t *testing.T) {
	f := New([]string{"main.go", "readme"})
	for _, r := range "zzz" {
		f.HandleKey(term.RuneKey(r))
	}
	if len(f.filtered) != 0 {
		t.Fatalf("zzz should match nothing, got %v", f.filtered)
	}
	for i := 0; i < 3; i++ {
		f.HandleKey(term.Named(term.SymBackspace))
	}
	if len(f.filtered) != 2 {
		t.Fatalf("backspace should restore matches, got %v", f.filtered)
	}
}

func TestFinderViewRenders(t *testing.T) {
	f := New([]string{"main.go", "cmd/root.go"})
	buf := cellbuf.New(40, 8)
	f.View(buf, cellbuf.Rect{X: 0, Y: 0, W: 40, H: 8}, true)
	out := buf.String()
	if !strings.Contains(out, "main.go") || !strings.Contains(out, "find file") {
		t.Fatalf("finder render wrong:\n%s", out)
	}
}
