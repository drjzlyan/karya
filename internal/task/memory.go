package task

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MEMORY.md is a task's agent-agnostic running memory — decisions, gotchas, and
// state accumulated across steps. It lives in the task dir (not with any agent),
// is folded into every agent prompt (internal/prompts), and any agent may append
// to it, so the agent working a task can be replaced mid-task without losing
// context (DESIGN.md §5).

// memoryHeader opens a fresh MEMORY.md.
const memoryHeader = "# Task memory\n\nAgent-agnostic notes for this task. Any agent may append; every prompt includes it.\n"

// MemoryPath returns the task's MEMORY.md path.
func (s *Store) MemoryPath(id string) string { return filepath.Join(s.Dir(id), "MEMORY.md") }

// Memory returns the task's MEMORY.md contents, or "" if none.
func (s *Store) Memory(id string) string {
	b, err := os.ReadFile(s.MemoryPath(id))
	if err != nil {
		return ""
	}
	return string(b)
}

// AppendMemory adds a timestamped, actor-attributed note to the task's MEMORY.md
// (creating it with a header on first write). Blank notes are ignored.
func (s *Store) AppendMemory(id, actor, note string) error {
	note = strings.TrimSpace(note)
	if note == "" {
		return nil
	}
	if err := os.MkdirAll(s.Dir(id), 0o755); err != nil {
		return err
	}
	path := s.MemoryPath(id)
	var b strings.Builder
	if _, err := os.Stat(path); os.IsNotExist(err) {
		b.WriteString(memoryHeader)
	}
	if actor == "" {
		actor = "unknown"
	}
	fmt.Fprintf(&b, "\n## %s — %s\n\n%s\n", time.Now().UTC().Format(time.RFC3339), actor, note)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(b.String())
	return err
}
