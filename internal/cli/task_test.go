package cli

import (
	"testing"

	"github.com/drjzlyan/karya/internal/task"
)

func TestParseTaskNewArgs(t *testing.T) {
	cases := []struct {
		name               string
		args               []string
		wantPrompt, wantAg string
	}{
		{"prompt only", []string{"add", "a", "hello", "endpoint"}, "add a hello endpoint", ""},
		{"agent after prompt", []string{"add hello", "--agent", "none"}, "add hello", "none"},
		{"agent before prompt", []string{"--agent", "claude", "fix", "bug"}, "fix bug", "claude"},
		{"agent equals form", []string{"do it", "--agent=codex"}, "do it", "codex"},
		{"single dash equals", []string{"do it", "-agent=crush"}, "do it", "crush"},
		{"empty", nil, "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotP, gotA := parseTaskNewArgs(c.args)
			if gotP != c.wantPrompt || gotA != c.wantAg {
				t.Errorf("parseTaskNewArgs(%v) = (%q, %q), want (%q, %q)", c.args, gotP, gotA, c.wantPrompt, c.wantAg)
			}
		})
	}
}

func TestParseTaskRemoveArgs(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantID  string
		wantYes bool
	}{
		{"id then flag", []string{"ab12", "-y"}, "ab12", true},
		{"flag then id", []string{"--yes", "ab12"}, "ab12", true},
		{"id only", []string{"ab12"}, "ab12", false},
		{"none", nil, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id, yes := parseTaskRemoveArgs(c.args)
			if id != c.wantID || yes != c.wantYes {
				t.Errorf("parseTaskRemoveArgs(%v) = (%q, %v), want (%q, %v)", c.args, id, yes, c.wantID, c.wantYes)
			}
		})
	}
}

func TestTaskSessionNameIsTmuxSafe(t *testing.T) {
	// tmux session names cannot contain '.' or ':'; a task id is hex so the
	// prefix is the only risk, but the contract must stay stable because
	// `karya task rm` looks a session up by this exact name to kill it.
	got := taskSessionName(task.Task{ID: "ab12cd34"})
	if got != "task-ab12cd34" {
		t.Errorf("taskSessionName = %q, want task-ab12cd34", got)
	}
}

func TestTaskSessionOptionsRootAtWorktree(t *testing.T) {
	tk := task.Task{ID: "x1", Worktree: "/state/worktrees/proj/x1", Agent: "claude"}
	o := taskSessionOptions(tk)
	if o.Workdir != tk.Worktree {
		t.Errorf("session workdir = %q, want the task worktree %q", o.Workdir, tk.Worktree)
	}
	if o.Agent != "claude" {
		t.Errorf("session agent = %q, want claude", o.Agent)
	}
	if o.Name != "task-x1" {
		t.Errorf("session name = %q, want task-x1", o.Name)
	}
}
