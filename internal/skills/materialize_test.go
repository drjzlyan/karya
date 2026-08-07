package skills

import (
	"os"
	"path/filepath"
	"testing"
)

// installFake puts a fake skill dir into the store.
func installFake(t *testing.T, store Store, name string) {
	t.Helper()
	dir := store.Path(name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMaterializeAndDematerialize(t *testing.T) {
	store := Store{Root: t.TempDir()}
	installFake(t, store, "retry")
	installFake(t, store, "lint")
	target := t.TempDir()

	if err := store.Materialize(target); err != nil {
		t.Fatal(err)
	}
	// Both skills are symlinks into the store.
	for _, name := range []string{"retry", "lint"} {
		link := filepath.Join(target, name)
		info, err := os.Lstat(link)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("%s not symlinked: %v", name, err)
		}
		// Resolves to the store copy.
		if b, err := os.ReadFile(filepath.Join(link, "SKILL.md")); err != nil || string(b) != "# "+name {
			t.Fatalf("link content wrong for %s: %q err=%v", name, b, err)
		}
	}

	// Dematerialize removes karya links only.
	foreign := filepath.Join(target, "user-skill")
	os.MkdirAll(foreign, 0o755) // a real dir the user owns
	if err := store.Dematerialize(target); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(target, "retry")); !os.IsNotExist(err) {
		t.Fatal("karya link should be removed")
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatal("user-owned entry must be left intact")
	}
}

func TestMaterializeLeavesForeignSymlink(t *testing.T) {
	store := Store{Root: t.TempDir()}
	installFake(t, store, "retry")
	target := t.TempDir()

	// A foreign symlink named like a skill, pointing elsewhere.
	other := t.TempDir()
	foreignLink := filepath.Join(target, "retry")
	if err := os.Symlink(other, foreignLink); err != nil {
		t.Fatal(err)
	}
	if err := store.Materialize(target); err != nil {
		t.Fatal(err)
	}
	// karya must not have clobbered the user's link.
	got, _ := os.Readlink(foreignLink)
	if got != other {
		t.Fatalf("foreign symlink was clobbered: %q", got)
	}
}

func TestUnlink(t *testing.T) {
	store := Store{Root: t.TempDir()}
	installFake(t, store, "retry")
	target := t.TempDir()
	store.Materialize(target)

	if err := store.Unlink(target, "retry"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(target, "retry")); !os.IsNotExist(err) {
		t.Fatal("unlink should remove the karya link")
	}
}
