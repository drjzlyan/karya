package tasksvc

import (
	"testing"
	"time"

	"github.com/drjzlyan/karya/internal/spec"
	"github.com/drjzlyan/karya/internal/task"
)

// plannedEnv returns an Env over a temp store holding one PLANNED task (awaiting
// the plan gate).
func plannedEnv(t *testing.T) (Env, string) {
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
	return Env{Store: store}, id
}

func TestNewTaskValidatesSlug(t *testing.T) {
	env := Env{Store: task.NewStore(t.TempDir())}
	if _, err := NewTask(env, "Bad Slug", ""); err == nil {
		t.Fatal("expected an error for an invalid slug")
	}
	tk, err := NewTask(env, "add-retries", "claude")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	if tk.State != task.StateDraft {
		t.Fatalf("new task state = %s, want draft", tk.State)
	}
	if tk.Agent != "claude" {
		t.Fatalf("new task agent = %q, want claude", tk.Agent)
	}
}

func TestCrossGateApprove(t *testing.T) {
	env, id := plannedEnv(t)
	if err := CrossGate(env, id, "human", false, ""); err != nil {
		t.Fatalf("approve: %v", err)
	}
	tk, _ := env.Store.Get(id)
	if tk.State != task.StateApproved {
		t.Fatalf("state = %s, want approved", tk.State)
	}
	last := tk.History[len(tk.History)-1]
	if last.Actor != "human" || last.Gate != task.GatePlan {
		t.Fatalf("crossing not recorded: %+v", last)
	}
}

func TestCrossGateRejectRequiresFeedback(t *testing.T) {
	env, id := plannedEnv(t)
	if err := CrossGate(env, id, "human", true, ""); err == nil {
		t.Fatal("reject without feedback should fail")
	}
	if tk, _ := env.Store.Get(id); tk.State != task.StatePlanned {
		t.Fatalf("state changed on failed reject: %s", tk.State)
	}
	if err := CrossGate(env, id, "human", true, "please redo the plan"); err != nil {
		t.Fatalf("reject with feedback: %v", err)
	}
	tk, _ := env.Store.Get(id)
	if tk.State != task.StateDraft {
		t.Fatalf("state = %s, want draft after reject", tk.State)
	}
	if tk.History[len(tk.History)-1].Feedback != "please redo the plan" {
		t.Fatalf("feedback not recorded: %+v", tk.History)
	}
}

func TestCrossGateNotPending(t *testing.T) {
	env := Env{Store: task.NewStore(t.TempDir())}
	id := task.NewID("x", time.Now())
	if _, err := env.Store.Create(id, spec.Template(id), ""); err != nil { // draft
		t.Fatalf("create: %v", err)
	}
	if err := CrossGate(env, id, "human", false, ""); err == nil {
		t.Fatal("crossing a non-pending task should fail")
	}
}
