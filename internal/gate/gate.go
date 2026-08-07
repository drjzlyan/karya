// Package gate is the thin, pure layer over the task state machine that answers
// "what human gate, if any, is this task waiting on, and where do approve/reject
// send it" (DESIGN.md §2). The state graph itself lives in internal/task; this
// package just names the pending gate so the CLI and review UX stay declarative.
package gate

import "github.com/drjzlyan/karya/internal/task"

// Pending describes the gate a task currently awaits and the states an approve
// or reject crossing moves it to.
type Pending struct {
	Gate    task.Gate
	Approve task.State // forward: the human/agent approved
	Reject  task.State // backward: sent to the agent with feedback
}

// For returns the pending gate for a state, or ok=false when the task is not
// waiting on a human gate (draft/approved/merging/done/abandoned).
func For(s task.State) (Pending, bool) {
	switch s {
	case task.StatePlanned:
		return Pending{Gate: task.GatePlan, Approve: task.StateApproved, Reject: task.StateDraft}, true
	case task.StateImplementing:
		return Pending{Gate: task.GateDiff, Approve: task.StateVerifying, Reject: task.StateApproved}, true
	case task.StateVerifying:
		return Pending{Gate: task.GateVerify, Approve: task.StateMerging, Reject: task.StateImplementing}, true
	}
	return Pending{}, false
}

// IsPending reports whether a task in this state awaits a human gate.
func IsPending(s task.State) bool {
	_, ok := For(s)
	return ok
}

// PendingTasks returns the tasks awaiting a human gate (the gate inbox).
func PendingTasks(tasks []task.Task) []task.Task {
	var out []task.Task
	for _, t := range tasks {
		if IsPending(t.State) {
			out = append(out, t)
		}
	}
	return out
}
