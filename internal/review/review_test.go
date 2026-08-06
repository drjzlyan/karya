package review

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/drjzlyan/karya/internal/spec"
	"github.com/drjzlyan/karya/internal/task"
)

// fakeDiffer returns a canned diff.
type fakeDiffer struct{ diff string }

func (f fakeDiffer) DiffRange(base, head string) (string, error) { return f.diff, nil }

func seedTask(t *testing.T) (*task.Store, string) {
	t.Helper()
	repo := t.TempDir()
	store := task.NewStore(repo)
	id := task.NewID("retry", time.Now())
	if _, err := store.Create(id, spec.Template(id), ""); err != nil {
		t.Fatalf("create task: %v", err)
	}
	return store, id
}

func TestAssembleBasics(t *testing.T) {
	store, id := seedTask(t)
	// Add a plan and evidence artifact.
	dir := store.Dir(id)
	os.WriteFile(filepath.Join(dir, "PLAN.md"), []byte("# Plan\nstep 1\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "VERIFY-1.md"), []byte("all green\n"), 0o644)

	r, err := Assemble(store, fakeDiffer{diff: "diff --git a b"}, id)
	if err != nil {
		t.Fatal(err)
	}
	if r.Spec == nil {
		t.Fatal("spec not loaded")
	}
	if r.Plan == "" {
		t.Fatal("plan not loaded")
	}
	if len(r.Evidence) != 1 || r.Evidence[0] != "all green\n" {
		t.Fatalf("evidence = %v", r.Evidence)
	}
	// Draft task has no branch, so no diff even though a differ is provided.
	if r.Diff != "" {
		t.Fatalf("draft task should have no diff, got %q", r.Diff)
	}
	// Draft is not awaiting a gate.
	if r.HasGate {
		t.Fatal("draft task should not be pending a gate")
	}
}

func TestAssembleDiffWhenBranchSet(t *testing.T) {
	store, id := seedTask(t)
	tk, _ := store.Get(id)
	tk.Base = "abc123"
	tk.Branch = "task/" + id
	tk.State = task.StatePlanned
	if err := store.Save(tk); err != nil {
		t.Fatal(err)
	}
	r, err := Assemble(store, fakeDiffer{diff: "DIFFDATA"}, id)
	if err != nil {
		t.Fatal(err)
	}
	if r.Diff != "DIFFDATA" {
		t.Fatalf("diff = %q", r.Diff)
	}
	if !r.HasGate || r.Pending.Gate != task.GatePlan {
		t.Fatalf("planned task should await the plan gate: %+v", r.Pending)
	}
}
