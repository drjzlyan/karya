package cli

import (
	"strings"
	"testing"

	"github.com/drjzlyan/karya/internal/task"
)

func TestParseTaskNewArgs(t *testing.T) {
	cases := []struct {
		name               string
		args               []string
		wantPrompt, wantAg string
		wantPlan           bool
	}{
		{"prompt only", []string{"add", "a", "hello", "endpoint"}, "add a hello endpoint", "", false},
		{"agent after prompt", []string{"add hello", "--agent", "none"}, "add hello", "none", false},
		{"agent before prompt", []string{"--agent", "claude", "fix", "bug"}, "fix bug", "claude", false},
		{"agent equals form", []string{"do it", "--agent=codex"}, "do it", "codex", false},
		{"single dash equals", []string{"do it", "-agent=crush"}, "do it", "crush", false},
		{"plan flag after prompt", []string{"build x", "--plan"}, "build x", "", true},
		{"plan and agent", []string{"--plan", "--agent", "claude", "do", "it"}, "do it", "claude", true},
		{"empty", nil, "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotP, gotA, gotPlan := parseTaskNewArgs(c.args)
			if gotP != c.wantPrompt || gotA != c.wantAg || gotPlan != c.wantPlan {
				t.Errorf("parseTaskNewArgs(%v) = (%q, %q, %v), want (%q, %q, %v)",
					c.args, gotP, gotA, gotPlan, c.wantPrompt, c.wantAg, c.wantPlan)
			}
		})
	}
}

func TestParseTaskRewindArgs(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		id, target string
		yes        bool
	}{
		{"id only", []string{"ab12"}, "ab12", "", false},
		{"id and index", []string{"ab12", "2"}, "ab12", "2", false},
		{"id target and yes anywhere", []string{"ab12", "-y", "deadbeef"}, "ab12", "deadbeef", true},
		{"none", nil, "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id, target, yes := parseTaskRewindArgs(c.args)
			if id != c.id || target != c.target || yes != c.yes {
				t.Errorf("parseTaskRewindArgs(%v) = (%q,%q,%v), want (%q,%q,%v)",
					c.args, id, target, yes, c.id, c.target, c.yes)
			}
		})
	}
}

func TestResolveCheckpoint(t *testing.T) {
	tk := task.Task{Checkpoints: []task.Checkpoint{
		{SHA: "aaaa1111", Label: "one"},
		{SHA: "bbbb2222", Label: "two"},
	}}
	if cp, _ := resolveCheckpoint(tk, ""); cp.Label != "two" {
		t.Errorf("empty target should pick latest; got %q", cp.Label)
	}
	if cp, _ := resolveCheckpoint(tk, "1"); cp.Label != "one" {
		t.Errorf("index 1 should pick first; got %q", cp.Label)
	}
	if cp, _ := resolveCheckpoint(tk, "bbbb"); cp.Label != "two" {
		t.Errorf("sha prefix should match; got %q", cp.Label)
	}
	if _, err := resolveCheckpoint(tk, "9"); err == nil {
		t.Error("out-of-range index should error")
	}
	if _, err := resolveCheckpoint(tk, "zzz"); err == nil {
		t.Error("unmatched sha should error")
	}
}

func TestAllowKeyAndShort(t *testing.T) {
	if k := allowKey("/home/u/p", "merge"); k != "allow./home/u/p.merge" {
		t.Errorf("allowKey = %q", k)
	}
	if short("deadbeefcafe") != "deadbeef" {
		t.Errorf("short = %q", short("deadbeefcafe"))
	}
	if short("abc") != "abc" {
		t.Errorf("short(short) = %q", short("abc"))
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

func TestDashboardChoice(t *testing.T) {
	tasks := []task.Task{{ID: "aa11"}, {ID: "bb22"}, {ID: "cc33"}}
	cases := []struct {
		in   string
		want int
	}{
		{"1", 0}, {"3", 2}, {"0", -1}, {"4", -1},
		{"bb22", 1}, {"cc", 2}, {"zz", -1},
	}
	for _, c := range cases {
		if got := dashboardChoice(c.in, tasks); got != c.want {
			t.Errorf("dashboardChoice(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestRenderTasksNumbered(t *testing.T) {
	tasks := []task.Task{
		{ID: "aa11", Status: task.StatusWorking, Agent: "claude", Title: "first"},
		{ID: "bb22", Status: task.StatusMerged, Agent: "none", Title: "second"},
	}
	numbered := renderTasks(tasks, true)
	if !strings.Contains(numbered, "#") || !strings.Contains(numbered, "1") || !strings.Contains(numbered, "aa11") {
		t.Errorf("numbered table missing index/id:\n%s", numbered)
	}
	plain := renderTasks(tasks, false)
	if strings.Contains(strings.SplitN(plain, "\n", 2)[0], "#") {
		t.Errorf("plain table should have no index column:\n%s", plain)
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
