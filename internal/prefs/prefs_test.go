package prefs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetMissingFile(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "prefs"))
	if got := s.Get("agent./x"); got != "" {
		t.Fatalf("Get on missing file = %q, want empty", got)
	}
}

func TestSetGetRoundTrip(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "sub", "prefs")) // parent dir created lazily
	if err := s.Set("agent./home/me/proj", "claude"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := s.Get("agent./home/me/proj"); got != "claude" {
		t.Fatalf("Get = %q, want claude", got)
	}
}

func TestSetReplacesAndPreservesOrder(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "prefs"))
	mustSet(t, s, "agent./a", "crush")
	mustSet(t, s, "agent./b", "codex")
	mustSet(t, s, "agent./a", "gemini") // replace first key

	if got := s.Get("agent./a"); got != "gemini" {
		t.Errorf("Get(/a) = %q, want gemini", got)
	}
	if got := s.Get("agent./b"); got != "codex" {
		t.Errorf("Get(/b) = %q, want codex", got)
	}
	// Replacement must not duplicate the key.
	got := s.Entries()
	if len(got) != 2 {
		t.Fatalf("Entries = %v, want 2 entries", got)
	}
	if got[0] != "agent./b=codex" {
		t.Errorf("order not preserved: %v", got)
	}
}

func TestValueWithEquals(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "prefs"))
	mustSet(t, s, "k", "a=b=c")
	if got := s.Get("k"); got != "a=b=c" {
		t.Fatalf("Get = %q, want a=b=c", got)
	}
}

func TestDelete(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "prefs"))
	mustSet(t, s, "agent./a", "crush")
	mustSet(t, s, "agent./b", "codex")
	if err := s.Delete("agent./a"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got := s.Get("agent./a"); got != "" {
		t.Errorf("Get after delete = %q, want empty", got)
	}
	if got := s.Get("agent./b"); got != "codex" {
		t.Errorf("unrelated key lost: %q", got)
	}
}

func TestDeleteMissingFileIsNoError(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "prefs"))
	if err := s.Delete("nope"); err != nil {
		t.Fatalf("Delete on missing file: %v", err)
	}
	if _, err := os.Stat(s.Path()); !os.IsNotExist(err) {
		t.Errorf("Delete created the file; want it left absent")
	}
}

func mustSet(t *testing.T, s *Store, k, v string) {
	t.Helper()
	if err := s.Set(k, v); err != nil {
		t.Fatalf("Set(%q,%q): %v", k, v, err)
	}
}
