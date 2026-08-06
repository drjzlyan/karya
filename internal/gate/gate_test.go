package gate

import (
	"testing"

	"github.com/drjzlyan/karya/internal/task"
)

func TestForStates(t *testing.T) {
	cases := []struct {
		state   task.State
		pending bool
		gate    task.Gate
		approve task.State
	}{
		{task.StateDraft, false, "", ""},
		{task.StatePlanned, true, task.GatePlan, task.StateApproved},
		{task.StateApproved, false, "", ""},
		{task.StateImplementing, true, task.GateDiff, task.StateVerifying},
		{task.StateVerifying, true, task.GateVerify, task.StateMerging},
		{task.StateMerging, false, "", ""},
		{task.StateDone, false, "", ""},
	}
	for _, c := range cases {
		p, ok := For(c.state)
		if ok != c.pending {
			t.Fatalf("%s pending = %v want %v", c.state, ok, c.pending)
		}
		if ok && (p.Gate != c.gate || p.Approve != c.approve) {
			t.Fatalf("%s pending = %+v want gate %s approve %s", c.state, p, c.gate, c.approve)
		}
	}
}

func TestPendingTasks(t *testing.T) {
	tasks := []task.Task{
		{ID: "a", State: task.StateDraft},
		{ID: "b", State: task.StatePlanned},
		{ID: "c", State: task.StateImplementing},
		{ID: "d", State: task.StateDone},
	}
	pending := PendingTasks(tasks)
	if len(pending) != 2 {
		t.Fatalf("want 2 pending, got %d", len(pending))
	}
	if pending[0].ID != "b" || pending[1].ID != "c" {
		t.Fatalf("wrong pending tasks: %+v", pending)
	}
}
