package task

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNewIDIsUniqueHex(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id := NewID()
		if len(id) != 8 {
			t.Fatalf("NewID() = %q, want 8 hex chars", id)
		}
		if seen[id] {
			t.Fatalf("NewID() collided on %q", id)
		}
		seen[id] = true
	}
}

func TestTitleFromPrompt(t *testing.T) {
	tests := []struct{ in, want string }{
		{"add a hello endpoint", "add a hello endpoint"},
		{"\n\n  first real line  \nsecond", "first real line"},
		{"", "untitled task"},
		{strings.Repeat("x", 80), strings.Repeat("x", 59) + "…"},
	}
	for _, tt := range tests {
		if got := TitleFromPrompt(tt.in); got != tt.want {
			t.Errorf("TitleFromPrompt(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCanTransition(t *testing.T) {
	ok := [][2]Status{
		{StatusAwaitingPlan, StatusWorking},
		{StatusWorking, StatusAwaitingReview},
		{StatusWorking, StatusMerged},
		{StatusAwaitingReview, StatusRejected},
		{StatusAwaitingReview, StatusWorking},
	}
	for _, p := range ok {
		if !CanTransition(p[0], p[1]) {
			t.Errorf("CanTransition(%s,%s) = false, want true", p[0], p[1])
		}
	}
	bad := [][2]Status{
		{StatusAwaitingPlan, StatusMerged}, // can't merge straight from planning
		{StatusMerged, StatusWorking},      // terminal
		{StatusRejected, StatusWorking},    // terminal
	}
	for _, p := range bad {
		if CanTransition(p[0], p[1]) {
			t.Errorf("CanTransition(%s,%s) = true, want false", p[0], p[1])
		}
	}
}

func TestCheckpointsRoundTrip(t *testing.T) {
	s := newStore(t)
	_, err := s.Save(Task{ID: "c1", Status: StatusWorking, Checkpoints: []Checkpoint{{SHA: "aaa", Label: "start"}}})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := s.Get("c1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.Checkpoints) != 1 || got.Checkpoints[0].SHA != "aaa" {
		t.Errorf("checkpoints did not round-trip: %+v", got.Checkpoints)
	}
}

func newStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(filepath.Join(t.TempDir(), "proj.json"))
}

func TestListEmptyWhenNoFile(t *testing.T) {
	got, err := newStore(t).List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("List() = %v, want empty", got)
	}
}

func TestSaveGetUpsertAndTimestamps(t *testing.T) {
	s := newStore(t)

	saved, err := s.Save(Task{ID: "a1", Title: "one", Status: StatusWorking})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if saved.Created.IsZero() || saved.Updated.IsZero() {
		t.Error("Save did not stamp Created/Updated")
	}

	// Upsert: same ID replaces, preserving Created.
	updated, err := s.Save(Task{ID: "a1", Title: "one-edited", Status: StatusAwaitingReview})
	if err != nil {
		t.Fatalf("Save upsert: %v", err)
	}
	if !updated.Created.Equal(saved.Created) {
		t.Errorf("upsert changed Created: %v → %v", saved.Created, updated.Created)
	}

	list, _ := s.List()
	if len(list) != 1 {
		t.Fatalf("after upsert len = %d, want 1", len(list))
	}
	got, err := s.Get("a1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "one-edited" || got.Status != StatusAwaitingReview {
		t.Errorf("Get returned stale task: %+v", got)
	}
}

func TestGetAndDeleteNotFound(t *testing.T) {
	s := newStore(t)
	if _, err := s.Get("nope"); err != ErrNotFound {
		t.Errorf("Get(missing) err = %v, want ErrNotFound", err)
	}
	if err := s.Delete("nope"); err != ErrNotFound {
		t.Errorf("Delete(missing) err = %v, want ErrNotFound", err)
	}
}

func TestDeleteRemoves(t *testing.T) {
	s := newStore(t)
	_, _ = s.Save(Task{ID: "a"})
	_, _ = s.Save(Task{ID: "b"})
	if err := s.Delete("a"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	list, _ := s.List()
	if len(list) != 1 || list[0].ID != "b" {
		t.Errorf("after Delete, list = %+v, want only b", list)
	}
}
