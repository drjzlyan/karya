package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path"
	"testing"
)

// mapSource is an in-memory Source for tests.
type mapSource map[string][]byte

func (m mapSource) Fetch(relpath string) ([]byte, error) {
	if b, ok := m[relpath]; ok {
		return b, nil
	}
	return nil, os.ErrNotExist
}

func sum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// buildRegistry returns a source + index for a single skill "retry".
func buildRegistry(t *testing.T) (mapSource, Entry) {
	t.Helper()
	skillMD := []byte("# retry\nRetry transient failures.\n")
	script := []byte("echo retry\n")
	entry := Entry{
		Name: "retry", Version: "1.0.0", Description: "Retry transient failures",
		Files: []File{
			{Path: "SKILL.md", SHA256: sum(skillMD)},
			{Path: "scripts/run.sh", SHA256: sum(script)},
		},
	}
	idx := Index{Skills: []Entry{entry}}
	idxJSON, _ := json.Marshal(idx)
	src := mapSource{
		"registry.json":        idxJSON,
		"retry/SKILL.md":       skillMD,
		"retry/scripts/run.sh": script,
	}
	return src, entry
}

func TestLoadIndexAndSearch(t *testing.T) {
	src, _ := buildRegistry(t)
	idx, err := LoadIndex(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Skills) != 1 {
		t.Fatalf("want 1 skill, got %d", len(idx.Skills))
	}
	if _, ok := idx.Find("retry"); !ok {
		t.Fatal("retry not found")
	}
	if len(idx.Search("transient")) != 1 {
		t.Fatal("search by description failed")
	}
	if len(idx.Search("nope")) != 0 {
		t.Fatal("search should not match")
	}
	if len(idx.Search("")) != 1 {
		t.Fatal("empty search should return all")
	}
}

func TestInstallVerifiesAndLands(t *testing.T) {
	src, entry := buildRegistry(t)
	store := Store{Root: t.TempDir()}

	if err := store.Install(entry, src); err != nil {
		t.Fatalf("install: %v", err)
	}
	if !store.Installed("retry") {
		t.Fatal("retry not installed")
	}
	if got := store.List(); len(got) != 1 || got[0] != "retry" {
		t.Fatalf("list = %v", got)
	}
	// Files landed with content.
	b, err := os.ReadFile(path.Join(store.Path("retry"), "scripts", "run.sh"))
	if err != nil || string(b) != "echo retry\n" {
		t.Fatalf("skill file wrong: %q err=%v", b, err)
	}
}

func TestInstallRejectsTamperedFile(t *testing.T) {
	src, entry := buildRegistry(t)
	src["retry/SKILL.md"] = []byte("TAMPERED") // hash no longer matches
	store := Store{Root: t.TempDir()}

	if err := store.Install(entry, src); err == nil {
		t.Fatal("install should fail on checksum mismatch")
	}
	// Atomic: nothing left behind.
	if store.Installed("retry") {
		t.Fatal("tampered install must not land a partial skill")
	}
	if entries, _ := os.ReadDir(store.Root); len(entries) != 0 {
		t.Fatalf("stage dir leaked: %v", entries)
	}
}

func TestRemove(t *testing.T) {
	src, entry := buildRegistry(t)
	store := Store{Root: t.TempDir()}
	if err := store.Install(entry, src); err != nil {
		t.Fatal(err)
	}
	if err := store.Remove("retry"); err != nil {
		t.Fatal(err)
	}
	if store.Installed("retry") {
		t.Fatal("remove failed")
	}
}

func TestDirSourceRejectsEscape(t *testing.T) {
	s := DirSource{Root: t.TempDir()}
	if _, err := s.Fetch("../etc/passwd"); err == nil {
		t.Fatal("path escape should be rejected")
	}
	if _, err := s.Fetch("/abs"); err == nil {
		t.Fatal("absolute path should be rejected")
	}
}
