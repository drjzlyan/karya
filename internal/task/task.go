// Package task is karya's task engine: the task is the unit of work in the
// human-in-the-loop IDE (DESIGN.md §2). A task is a spec contract
// (internal/spec), a state machine with mandatory human gates, and an audit
// trail — all persisted inside the user's repository at
// .karya/tasks/<id>/ (SPEC.md + STATE.json), so state survives restarts and is
// diffable and greppable by humans and re-ingestible by agents.
//
// Tasks branch and work in isolated git worktrees (internal/worktree); the
// user's working tree is never the agent's working tree. Only SPEC.md is meant
// to be committed (DESIGN.md §15): EnsureProjectDir installs a .karya/.gitignore
// that keeps the rest of the runtime state local.
package task

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/drjzlyan/karya/internal/spec"
)

// State is a task's position in the gate state machine (DESIGN.md §2):
//
//	DRAFT → PLANNED ─gate:plan→ APPROVED → IMPLEMENTING ─gate:diff→
//	VERIFYING ─gate:verify→ MERGING → DONE
//
// Rejection at a gate loops the task back with the human's feedback; ABANDONED
// is the terminal state for torn-down tasks.
type State string

// The task lifecycle states.
const (
	StateDraft        State = "draft"
	StatePlanned      State = "planned"
	StateApproved     State = "approved"
	StateImplementing State = "implementing"
	StateVerifying    State = "verifying"
	StateMerging      State = "merging"
	StateDone         State = "done"
	StateAbandoned    State = "abandoned"
)

// Gate is a mandatory human (or recorded-delegation) approval point.
type Gate string

// The three gates between states.
const (
	GatePlan   Gate = "plan"
	GateDiff   Gate = "diff"
	GateVerify Gate = "verify"
)

// GateFor reports which gate a transition between two states crosses, if any.
// APPROVED → IMPLEMENTING and MERGING → DONE cross no gate; every other
// forward transition and every rejection does.
func GateFor(from, to State) Gate {
	switch {
	case from == StatePlanned && (to == StateApproved || to == StateDraft):
		return GatePlan
	case from == StateImplementing && (to == StateVerifying || to == StateApproved):
		return GateDiff
	case from == StateVerifying && (to == StateMerging || to == StateImplementing):
		return GateVerify
	}
	return ""
}

// transitions is the allowed state graph. Forward moves advance the workflow;
// backward moves are gate rejections that return the task to the agent with
// feedback. DONE and ABANDONED are terminal.
var transitions = map[State][]State{
	StateDraft:        {StatePlanned, StateAbandoned},
	StatePlanned:      {StateApproved, StateDraft, StateAbandoned},
	StateApproved:     {StateImplementing, StateAbandoned},
	StateImplementing: {StateVerifying, StateApproved, StateAbandoned},
	StateVerifying:    {StateMerging, StateImplementing, StateAbandoned},
	StateMerging:      {StateDone, StateAbandoned},
}

// CanTransition reports whether a task may move from state from to state to.
func CanTransition(from, to State) bool {
	for _, s := range transitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// Transition is one recorded state crossing — the audit trail entry that
// answers "who approved this, when, with what feedback" (DESIGN.md §13).
type Transition struct {
	At       time.Time `json:"at"`
	From     State     `json:"from"`
	To       State     `json:"to"`
	Gate     Gate      `json:"gate,omitempty"`
	Actor    string    `json:"actor"` // "human" or "agent:<name>" for delegated crossings
	Feedback string    `json:"feedback,omitempty"`
}

// Task is one unit of work: its state, its isolated workspace, and its history.
// It is the STATE.json shadow of the task directory.
type Task struct {
	ID       string       `json:"id"`
	State    State        `json:"state"`
	Agent    string       `json:"agent,omitempty"`    // preferred agent (from the spec)
	Base     string       `json:"base,omitempty"`     // base commit the task branch forked from
	Branch   string       `json:"branch,omitempty"`   // task/<id>, set by `task start`
	Worktree string       `json:"worktree,omitempty"` // isolated checkout path, set by `task start`
	History  []Transition `json:"history,omitempty"`
	Created  time.Time    `json:"created"`
	Updated  time.Time    `json:"updated"`
}

// rejections are the backward moves: a gate sending the task back to the agent
// with the human's feedback appended (DESIGN.md §2 — the core HITL loop).
var rejections = map[State]State{
	StatePlanned:      StateDraft,
	StateImplementing: StateApproved,
	StateVerifying:    StateImplementing,
}

// IsRejection reports whether moving from state from to state to is a gate
// rejection (a backward move) rather than forward progress or an abandon.
func IsRejection(from, to State) bool { return rejections[from] == to }

// Transition moves t to state to, recording the crossing in the history. It
// rejects illegal moves, actor-less crossings (every crossing names who or what
// approved it — the history is the audit trail), and rejections without
// feedback: the feedback is what the agent revises against, so a silent
// rejection is a bug in the loop.
func (t *Task) Transition(to State, actor, feedback string) error {
	if !CanTransition(t.State, to) {
		return fmt.Errorf("task: illegal transition %s → %s", t.State, to)
	}
	if actor == "" {
		return fmt.Errorf("task: transition %s → %s must name its actor", t.State, to)
	}
	if IsRejection(t.State, to) && feedback == "" {
		return fmt.Errorf("task: rejection %s → %s requires feedback", t.State, to)
	}
	gate := GateFor(t.State, to)
	t.History = append(t.History, Transition{
		At: time.Now().UTC(), From: t.State, To: to,
		Gate: gate, Actor: actor, Feedback: feedback,
	})
	t.State = to
	t.Updated = time.Now().UTC()
	return nil
}

// ErrNotFound is returned by Store.Get/Spec when no task has the given id.
var ErrNotFound = errors.New("task not found")

// ErrExists is returned by Store.Create when a task with the id already exists.
var ErrExists = errors.New("task already exists")

// NewID derives a task id from a slug: date-prefixed so `task list` reads
// chronologically, e.g. 2026-08-05-add-retry-to-downloader (DESIGN.md §3).
func NewID(slug string, now time.Time) string {
	return now.Format("2006-01-02") + "-" + slug
}

// ProjectDir is the karya directory inside a repository.
func ProjectDir(repoRoot string) string { return filepath.Join(repoRoot, ".karya") }

// karyaGitIgnore keeps karya runtime state out of the user's git status while
// leaving task specs committable — SPEC.md is the contract (DESIGN.md §15).
const karyaGitIgnore = `# karya runtime state is local; task specs (the contract) stay committable.
*
!*/
!**/SPEC.md
`

// EnsureProjectDir creates .karya/tasks and installs .karya/.gitignore if
// missing. Idempotent; never overwrites an existing .gitignore.
func EnsureProjectDir(repoRoot string) error {
	if err := os.MkdirAll(filepath.Join(ProjectDir(repoRoot), "tasks"), 0o755); err != nil {
		return fmt.Errorf("task: create .karya: %w", err)
	}
	gi := filepath.Join(ProjectDir(repoRoot), ".gitignore")
	if _, err := os.Stat(gi); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(gi, []byte(karyaGitIgnore), 0o644); err != nil {
			return fmt.Errorf("task: write .karya/.gitignore: %w", err)
		}
	}
	return nil
}

// Store persists the tasks of one repository as per-task directories under
// <repo>/.karya/tasks/. Construct one with NewStore.
type Store struct {
	repoRoot string
}

// NewStore returns a Store rooted at the repository's .karya directory.
func NewStore(repoRoot string) *Store { return &Store{repoRoot: repoRoot} }

// tasksDir is where per-task directories live.
func (s *Store) tasksDir() string { return filepath.Join(ProjectDir(s.repoRoot), "tasks") }

// Dir returns the task directory for id.
func (s *Store) Dir(id string) string { return filepath.Join(s.tasksDir(), id) }

// SpecPath returns the path of a task's SPEC.md.
func (s *Store) SpecPath(id string) string { return filepath.Join(s.Dir(id), "SPEC.md") }

// statePath returns the path of a task's STATE.json.
func (s *Store) statePath(id string) string { return filepath.Join(s.Dir(id), "STATE.json") }

// Create scaffolds a new task: the spec document and an initial STATE.json at
// draft. It returns ErrExists if the id is taken. The spec is stored as
// rendered — the template is expected to be filled in by the human before the
// task advances.
func (s *Store) Create(id string, doc string, agentName string) (Task, error) {
	if _, err := os.Stat(s.Dir(id)); err == nil {
		return Task{}, fmt.Errorf("task: %s: %w", id, ErrExists)
	}
	if err := EnsureProjectDir(s.repoRoot); err != nil {
		return Task{}, err
	}
	if err := os.MkdirAll(s.Dir(id), 0o755); err != nil {
		return Task{}, fmt.Errorf("task: create %s: %w", id, err)
	}
	if err := os.WriteFile(s.SpecPath(id), []byte(doc), 0o644); err != nil {
		return Task{}, fmt.Errorf("task: write spec: %w", err)
	}
	now := time.Now().UTC()
	t := Task{ID: id, State: StateDraft, Agent: agentName, Created: now, Updated: now}
	if err := s.Save(t); err != nil {
		return Task{}, err
	}
	return t, nil
}

// Get loads the task with id, or ErrNotFound.
func (s *Store) Get(id string) (Task, error) {
	data, err := os.ReadFile(s.statePath(id))
	if errors.Is(err, os.ErrNotExist) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, fmt.Errorf("task: read state: %w", err)
	}
	var t Task
	if err := json.Unmarshal(data, &t); err != nil {
		return Task{}, fmt.Errorf("task: parse %s: %w", s.statePath(id), err)
	}
	return t, nil
}

// Spec loads and parses a task's SPEC.md.
func (s *Store) Spec(id string) (*spec.Spec, error) {
	data, err := os.ReadFile(s.SpecPath(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("task: read spec: %w", err)
	}
	return spec.Parse(data)
}

// List returns every task, sorted by id (chronological, since ids are
// date-prefixed). A missing tasks dir is an empty list, not an error.
func (s *Store) List() ([]Task, error) {
	entries, err := os.ReadDir(s.tasksDir())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("task: list: %w", err)
	}
	var out []Task
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		t, err := s.Get(e.Name())
		if err != nil {
			continue // a task dir without valid STATE.json is not a task
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// Save writes t's STATE.json, stamping Updated.
func (s *Store) Save(t Task) error {
	t.Updated = time.Now().UTC()
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return fmt.Errorf("task: encode state: %w", err)
	}
	if err := os.WriteFile(s.statePath(t.ID), data, 0o644); err != nil {
		return fmt.Errorf("task: write state: %w", err)
	}
	return nil
}

// Delete removes the task directory (spec, state, artifacts). It is the
// teardown half of `task abandon`; the worktree/branch teardown lives in
// internal/worktree. ErrNotFound if no such task exists.
func (s *Store) Delete(id string) error {
	if _, err := os.Stat(s.Dir(id)); errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}
	if err := os.RemoveAll(s.Dir(id)); err != nil {
		return fmt.Errorf("task: remove %s: %w", id, err)
	}
	return nil
}

// Summary is the per-state count board `karya task status` renders.
type Summary struct {
	Counts  map[State]int
	Total   int
	Pending []Task // tasks waiting on a human gate crossing
}

// gatePendingStates are the states whose next forward move crosses a human gate.
var gatePendingStates = map[State]bool{
	StatePlanned:      true,
	StateImplementing: true,
	StateVerifying:    true,
}

// Summarize counts tasks per state and collects the gate inbox: tasks parked
// at a state whose forward transition needs a human approval.
func Summarize(tasks []Task) Summary {
	sum := Summary{Counts: map[State]int{}, Total: len(tasks)}
	for _, t := range tasks {
		sum.Counts[t.State]++
		if gatePendingStates[t.State] {
			sum.Pending = append(sum.Pending, t)
		}
	}
	return sum
}

// Title reads the first line of a spec objective as a human-readable task
// title for list/board rendering.
func Title(s *spec.Spec) string {
	if s == nil {
		return ""
	}
	line, _, _ := strings.Cut(s.Objective, "\n")
	const max = 60
	if len(line) > max {
		return line[:max-1] + "…"
	}
	return line
}
