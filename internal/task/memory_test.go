package task

import (
	"strings"
	"testing"
	"time"

	"github.com/drjzlyan/karya/internal/spec"
)

func TestAppendAndReadMemory(t *testing.T) {
	store := NewStore(t.TempDir())
	id := NewID("retry", time.Now())
	if _, err := store.Create(id, spec.Template(id), ""); err != nil {
		t.Fatalf("create: %v", err)
	}

	if m := store.Memory(id); m != "" {
		t.Fatalf("fresh task should have no memory, got %q", m)
	}
	if err := store.AppendMemory(id, "agent:claude", "chose exponential backoff"); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendMemory(id, "human", "cap retries at 3"); err != nil {
		t.Fatal(err)
	}

	m := store.Memory(id)
	if !strings.Contains(m, "# Task memory") {
		t.Fatalf("memory missing header:\n%s", m)
	}
	for _, want := range []string{"agent:claude", "exponential backoff", "human", "cap retries at 3"} {
		if !strings.Contains(m, want) {
			t.Fatalf("memory missing %q:\n%s", want, m)
		}
	}
	// Order preserved (append-only).
	if strings.Index(m, "exponential backoff") >= strings.Index(m, "cap retries at 3") {
		t.Fatalf("memory entries out of order:\n%s", m)
	}
}

func TestAppendMemoryIgnoresBlank(t *testing.T) {
	store := NewStore(t.TempDir())
	id := NewID("x", time.Now())
	store.Create(id, spec.Template(id), "")
	if err := store.AppendMemory(id, "human", "   "); err != nil {
		t.Fatal(err)
	}
	if store.Memory(id) != "" {
		t.Fatal("blank note should not create memory")
	}
}
