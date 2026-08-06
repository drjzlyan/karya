package cli

import (
	"testing"
	"time"

	"github.com/drjzlyan/karya/internal/spec"
	"github.com/drjzlyan/karya/internal/task"
)

func plannedTask(t *testing.T) (*task.Store, string) {
	t.Helper()
	store := task.NewStore(t.TempDir())
	id := task.NewID("retry", time.Now())
	if _, err := store.Create(id, spec.Template(id), ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	tk, _ := store.Get(id)
	tk.State = task.StatePlanned
	if err := store.Save(tk); err != nil {
		t.Fatalf("save: %v", err)
	}
	return store, id
}

func TestCrossApprove(t *testing.T) {
	store, id := plannedTask(t)
	if rc := cross(store, id, "human", false, ""); rc != 0 {
		t.Fatalf("approve rc = %d", rc)
	}
	tk, _ := store.Get(id)
	if tk.State != task.StateApproved {
		t.Fatalf("state = %s want approved", tk.State)
	}
	last := tk.History[len(tk.History)-1]
	if last.Actor != "human" || last.Gate != task.GatePlan {
		t.Fatalf("crossing not recorded: %+v", last)
	}
}

func TestCrossRejectRequiresFeedback(t *testing.T) {
	store, id := plannedTask(t)
	if rc := cross(store, id, "human", true, ""); rc != 2 {
		t.Fatalf("reject without feedback should fail (rc=2), got %d", rc)
	}
	// state unchanged
	if tk, _ := store.Get(id); tk.State != task.StatePlanned {
		t.Fatalf("state changed on failed reject: %s", tk.State)
	}
	if rc := cross(store, id, "human", true, "please redo the plan"); rc != 0 {
		t.Fatalf("reject with feedback rc = %d", rc)
	}
	tk, _ := store.Get(id)
	if tk.State != task.StateDraft {
		t.Fatalf("state = %s want draft after reject", tk.State)
	}
	if tk.History[len(tk.History)-1].Feedback != "please redo the plan" {
		t.Fatalf("feedback not recorded: %+v", tk.History)
	}
}

func TestCrossDelegateRecordsAgent(t *testing.T) {
	store, id := plannedTask(t)
	if rc := cross(store, id, "agent:gemini", false, ""); rc != 0 {
		t.Fatalf("delegate rc = %d", rc)
	}
	tk, _ := store.Get(id)
	if tk.History[len(tk.History)-1].Actor != "agent:gemini" {
		t.Fatalf("delegation actor not recorded: %+v", tk.History)
	}
}

func TestCrossNotPending(t *testing.T) {
	store := task.NewStore(t.TempDir())
	id := task.NewID("x", time.Now())
	store.Create(id, spec.Template(id), "") // draft
	if rc := cross(store, id, "human", false, ""); rc != 2 {
		t.Fatalf("crossing a non-pending task should fail (rc=2), got %d", rc)
	}
}
