package task

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/drjzlyan/karya/internal/spec"
)

func TestNewID(t *testing.T) {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	if got := NewID("add-retry", now); got != "2026-08-05-add-retry" {
		t.Errorf("NewID = %q", got)
	}
}

func TestTransitionGraph(t *testing.T) {
	forward := [][2]State{
		{StateDraft, StatePlanned},
		{StatePlanned, StateApproved},
		{StateApproved, StateImplementing},
		{StateImplementing, StateVerifying},
		{StateVerifying, StateMerging},
		{StateMerging, StateDone},
	}
	for _, tr := range forward {
		if !CanTransition(tr[0], tr[1]) {
			t.Errorf("forward %s → %s should be allowed", tr[0], tr[1])
		}
		if IsRejection(tr[0], tr[1]) {
			t.Errorf("forward %s → %s flagged as rejection", tr[0], tr[1])
		}
	}
	backward := [][2]State{
		{StatePlanned, StateDraft},
		{StateImplementing, StateApproved},
		{StateVerifying, StateImplementing},
	}
	for _, tr := range backward {
		if !CanTransition(tr[0], tr[1]) {
			t.Errorf("rejection %s → %s should be allowed", tr[0], tr[1])
		}
		if !IsRejection(tr[0], tr[1]) {
			t.Errorf("%s → %s should be a rejection", tr[0], tr[1])
		}
	}
	illegal := [][2]State{
		{StateDraft, StateApproved},   // skips the plan gate
		{StateDraft, StateDone},       // skips everything
		{StateApproved, StateDraft},   // not a rejection pair
		{StateDone, StateDraft},       // terminal
		{StateAbandoned, StateDraft},  // terminal
		{StateMerging, StateApproved}, // not a rejection pair
	}
	for _, tr := range illegal {
		if CanTransition(tr[0], tr[1]) {
			t.Errorf("%s → %s should be illegal", tr[0], tr[1])
		}
	}
	// Every non-terminal state can be abandoned.
	for _, from := range []State{StateDraft, StatePlanned, StateApproved, StateImplementing, StateVerifying, StateMerging} {
		if !CanTransition(from, StateAbandoned) {
			t.Errorf("%s → abandoned should be allowed", from)
		}
	}
}

func TestGateFor(t *testing.T) {
	cases := []struct {
		from, to State
		want     Gate
	}{
		{StateDraft, StatePlanned, ""},
		{StatePlanned, StateApproved, GatePlan},
		{StatePlanned, StateDraft, GatePlan},
		{StateApproved, StateImplementing, ""},
		{StateImplementing, StateVerifying, GateDiff},
		{StateImplementing, StateApproved, GateDiff},
		{StateVerifying, StateMerging, GateVerify},
		{StateVerifying, StateImplementing, GateVerify},
		{StateMerging, StateDone, ""},
	}
	for _, c := range cases {
		if got := GateFor(c.from, c.to); got != c.want {
			t.Errorf("GateFor(%s, %s) = %q, want %q", c.from, c.to, got, c.want)
		}
	}
}

func TestTransitionRecordsHistory(t *testing.T) {
	task := Task{ID: "x", State: StateDraft}
	if err := task.Transition(StatePlanned, "agent:claude", ""); err != nil {
		t.Fatalf("Transition: %v", err)
	}
	if task.State != StatePlanned {
		t.Fatalf("State = %s", task.State)
	}
	if err := task.Transition(StateApproved, "human", ""); err != nil {
		t.Fatalf("Transition: %v", err)
	}
	if len(task.History) != 2 {
		t.Fatalf("History = %d entries", len(task.History))
	}
	gate := task.History[1]
	if gate.Gate != GatePlan || gate.Actor != "human" || gate.From != StatePlanned || gate.To != StateApproved {
		t.Errorf("gate entry = %+v", gate)
	}
	if gate.At.IsZero() {
		t.Error("gate entry has no timestamp")
	}
}

func TestTransitionErrors(t *testing.T) {
	t.Run("illegal", func(t *testing.T) {
		task := Task{State: StateDraft}
		if err := task.Transition(StateDone, "human", ""); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("actor required", func(t *testing.T) {
		task := Task{State: StateDraft}
		if err := task.Transition(StatePlanned, "", ""); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("rejection requires feedback", func(t *testing.T) {
		task := Task{State: StatePlanned}
		if err := task.Transition(StateDraft, "human", ""); err == nil {
			t.Fatal("want error")
		}
		if err := task.Transition(StateDraft, "human", "plan too vague"); err != nil {
			t.Fatalf("Transition: %v", err)
		}
		if task.History[0].Feedback != "plan too vague" {
			t.Errorf("feedback not recorded: %+v", task.History[0])
		}
	})
}

func TestStoreLifecycle(t *testing.T) {
	repo := t.TempDir()
	store := NewStore(repo)

	if _, err := store.List(); err != nil {
		t.Fatalf("List on missing dir: %v", err)
	}

	doc := spec.Template("2026-08-05-demo")
	created, err := store.Create("2026-08-05-demo", doc, "claude")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.State != StateDraft || created.Agent != "claude" {
		t.Errorf("created = %+v", created)
	}
	if _, err := os.Stat(store.SpecPath("2026-08-05-demo")); err != nil {
		t.Errorf("SPEC.md not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ProjectDir(repo), ".gitignore")); err != nil {
		t.Errorf(".karya/.gitignore not installed: %v", err)
	}

	if _, err := store.Create("2026-08-05-demo", doc, ""); !errors.Is(err, ErrExists) {
		t.Errorf("duplicate Create err = %v, want ErrExists", err)
	}

	// State survives a "restart": a fresh Store over the same repo sees it.
	loaded, err := NewStore(repo).Get("2026-08-05-demo")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if loaded.ID != created.ID || loaded.State != StateDraft {
		t.Errorf("loaded = %+v", loaded)
	}

	if err := loaded.Transition(StatePlanned, "agent:claude", ""); err != nil {
		t.Fatalf("Transition: %v", err)
	}
	if err := store.Save(loaded); err != nil {
		t.Fatalf("Save: %v", err)
	}
	reloaded, err := store.Get("2026-08-05-demo")
	if err != nil {
		t.Fatalf("Get after save: %v", err)
	}
	if reloaded.State != StatePlanned || len(reloaded.History) != 1 {
		t.Errorf("reloaded = %+v", reloaded)
	}
	if reloaded.Created.IsZero() || reloaded.Updated.IsZero() {
		t.Error("timestamps not persisted")
	}

	sp, err := store.Spec("2026-08-05-demo")
	if err != nil {
		t.Fatalf("Spec: %v", err)
	}
	if sp.ID != "2026-08-05-demo" {
		t.Errorf("spec id = %q", sp.ID)
	}

	tasks, err := store.List()
	if err != nil || len(tasks) != 1 {
		t.Fatalf("List = %v, %v", tasks, err)
	}

	if err := store.Delete("2026-08-05-demo"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get("2026-08-05-demo"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after Delete err = %v, want ErrNotFound", err)
	}
	if err := store.Delete("2026-08-05-demo"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete again err = %v, want ErrNotFound", err)
	}
}

func TestStoreListSortedAndSkipsInvalid(t *testing.T) {
	repo := t.TempDir()
	store := NewStore(repo)
	for _, id := range []string{"2026-08-05-b", "2026-08-01-a"} {
		if _, err := store.Create(id, spec.Template(id), ""); err != nil {
			t.Fatalf("Create %s: %v", id, err)
		}
	}
	// A task dir without STATE.json is skipped, not fatal.
	if err := os.MkdirAll(store.Dir("junk"), 0o755); err != nil {
		t.Fatal(err)
	}
	tasks, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != 2 || tasks[0].ID != "2026-08-01-a" || tasks[1].ID != "2026-08-05-b" {
		t.Errorf("List = %v", tasks)
	}
}

func TestEnsureProjectDirDoesNotClobberGitignore(t *testing.T) {
	repo := t.TempDir()
	if err := EnsureProjectDir(repo); err != nil {
		t.Fatal(err)
	}
	gi := filepath.Join(ProjectDir(repo), ".gitignore")
	if err := os.WriteFile(gi, []byte("custom\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureProjectDir(repo); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(gi)
	if string(data) != "custom\n" {
		t.Errorf("gitignore clobbered: %q", data)
	}
}

func TestSummarize(t *testing.T) {
	tasks := []Task{
		{ID: "a", State: StateDraft},
		{ID: "b", State: StatePlanned},
		{ID: "c", State: StateVerifying},
		{ID: "d", State: StateDone},
	}
	sum := Summarize(tasks)
	if sum.Total != 4 || sum.Counts[StateDraft] != 1 || sum.Counts[StateDone] != 1 {
		t.Errorf("summary = %+v", sum)
	}
	if len(sum.Pending) != 2 || sum.Pending[0].ID != "b" || sum.Pending[1].ID != "c" {
		t.Errorf("pending = %+v", sum.Pending)
	}
}

func TestTitle(t *testing.T) {
	if got := Title(nil); got != "" {
		t.Errorf("Title(nil) = %q", got)
	}
	s := &spec.Spec{Objective: "Short objective.\nMore detail."}
	if got := Title(s); got != "Short objective." {
		t.Errorf("Title = %q", got)
	}
	long := &spec.Spec{Objective: "...................................................................................................."}
	if got := Title(long); len([]rune(got)) != 60 {
		t.Errorf("truncated Title len = %d", len([]rune(got)))
	}
}
