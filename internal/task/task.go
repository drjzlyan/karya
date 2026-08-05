// Package task models karya's unit of agent work and persists it. A Task is the
// primary noun of the human-in-the-loop, agents-first IDE: a prompt handed to an
// agent that runs in its own isolated git worktree (internal/worktree) and moves
// through a review lifecycle the human drives. The Store keeps tasks in a
// per-project JSON file under the karya prefix (config.Paths.TasksDir), so — like
// all karya state — they never touch the user's own config.
package task

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Status is a task's position in the review lifecycle:
//
//	planning → awaiting-plan → working → awaiting-review → merged | rejected
//
// The plan states (planning, awaiting-plan) are only entered when a task is
// created with plan-approval requested; otherwise a task starts at working.
type Status string

// The task lifecycle states.
const (
	StatusPlanning       Status = "planning"
	StatusAwaitingPlan   Status = "awaiting-plan"
	StatusWorking        Status = "working"
	StatusAwaitingReview Status = "awaiting-review"
	StatusMerged         Status = "merged"
	StatusRejected       Status = "rejected"
)

// ErrNotFound is returned by Store.Get/Delete when no task has the given id.
var ErrNotFound = errors.New("task not found")

// Checkpoint is a restorable snapshot of a task's worktree: a commit on the task
// branch that `karya task rewind` can reset back to.
type Checkpoint struct {
	SHA     string    `json:"sha"`
	Label   string    `json:"label"`
	Created time.Time `json:"created"`
}

// Task is one unit of agent work and its isolated workspace.
type Task struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Prompt      string       `json:"prompt"`
	Agent       string       `json:"agent"`
	Status      Status       `json:"status"`
	Branch      string       `json:"branch"`   // namespaced git branch, e.g. karya/<id>
	Worktree    string       `json:"worktree"` // isolated checkout path (karya-owned)
	Repo        string       `json:"repo"`     // repository top-level the task belongs to
	Plan        string       `json:"plan,omitempty"`
	BaseCommit  string       `json:"base_commit,omitempty"` // repo HEAD the branch forked from
	Checkpoints []Checkpoint `json:"checkpoints,omitempty"` // rewind targets, oldest first
	Created     time.Time    `json:"created"`
	Updated     time.Time    `json:"updated"`
}

// transitions is the allowed forward status graph. Merged and Rejected are
// terminal. It keeps the review lifecycle honest — a task cannot, say, be merged
// straight out of planning without passing through work.
var transitions = map[Status][]Status{
	StatusPlanning:       {StatusAwaitingPlan, StatusWorking, StatusRejected},
	StatusAwaitingPlan:   {StatusWorking, StatusRejected},
	StatusWorking:        {StatusAwaitingReview, StatusMerged, StatusRejected},
	StatusAwaitingReview: {StatusMerged, StatusRejected, StatusWorking},
}

// CanTransition reports whether a task may move from status from to status to.
func CanTransition(from, to Status) bool {
	for _, s := range transitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// NewID returns a short, unique task id (8 hex chars) suitable for use in a
// branch name and a directory segment.
func NewID() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fall back to a timestamp; collisions are astronomically unlikely either
		// way and a task id is never security-sensitive.
		return fmt.Sprintf("%08x", time.Now().UnixNano()&0xffffffff)
	}
	return hex.EncodeToString(b[:])
}

// TitleFromPrompt derives a concise, single-line title from a prompt: its first
// non-blank line, trimmed and truncated to a readable length.
func TitleFromPrompt(prompt string) string {
	for _, line := range strings.Split(prompt, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			const max = 60
			if len(s) > max {
				return s[:max-1] + "…"
			}
			return s
		}
	}
	return "untitled task"
}

// Store persists the tasks of a single project as one JSON file. Construct one
// with NewStore; the file and its parent directory are created lazily on Save.
type Store struct {
	path string
}

// NewStore returns a Store backed by the JSON file at path. Callers name the file
// per project (typically TasksDir/<worktree.ProjectSlug>.json) so each project's
// tasks are kept together.
func NewStore(path string) *Store { return &Store{path: path} }

// List returns all tasks in creation order (oldest first), or an empty slice if
// the store file does not exist yet.
func (s *Store) List() ([]Task, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("task: read store: %w", err)
	}
	var tasks []Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("task: parse store %s: %w", s.path, err)
	}
	return tasks, nil
}

// Get returns the task with id, or ErrNotFound.
func (s *Store) Get(id string) (Task, error) {
	tasks, err := s.List()
	if err != nil {
		return Task{}, err
	}
	for _, t := range tasks {
		if t.ID == id {
			return t, nil
		}
	}
	return Task{}, ErrNotFound
}

// Save upserts t (matched by ID), stamping Updated (and Created on first save),
// and writes the store. It returns the stored task.
func (s *Store) Save(t Task) (Task, error) {
	tasks, err := s.List()
	if err != nil {
		return Task{}, err
	}
	now := time.Now().UTC()
	t.Updated = now
	replaced := false
	for i, existing := range tasks {
		if existing.ID == t.ID {
			t.Created = existing.Created
			tasks[i] = t
			replaced = true
			break
		}
	}
	if !replaced {
		if t.Created.IsZero() {
			t.Created = now
		}
		tasks = append(tasks, t)
	}
	if err := s.write(tasks); err != nil {
		return Task{}, err
	}
	return t, nil
}

// Delete removes the task with id. It returns ErrNotFound if no such task exists.
func (s *Store) Delete(id string) error {
	tasks, err := s.List()
	if err != nil {
		return err
	}
	kept := tasks[:0]
	found := false
	for _, t := range tasks {
		if t.ID == id {
			found = true
			continue
		}
		kept = append(kept, t)
	}
	if !found {
		return ErrNotFound
	}
	return s.write(kept)
}

// write serializes tasks to the store file, creating the parent dir as needed.
func (s *Store) write(tasks []Task) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("task: create dir: %w", err)
	}
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("task: encode store: %w", err)
	}
	if err := os.WriteFile(s.path, data, 0o644); err != nil {
		return fmt.Errorf("task: write %s: %w", s.path, err)
	}
	return nil
}
