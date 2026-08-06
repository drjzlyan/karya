package cli

import (
	"strings"
	"testing"
	"text/tabwriter"

	"github.com/drjzlyan/karya/internal/task"
)

func TestParseIDYesArgs(t *testing.T) {
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
			id, yes := parseIDYesArgs(c.args)
			if id != c.wantID || yes != c.wantYes {
				t.Errorf("parseIDYesArgs(%v) = (%q, %v), want (%q, %v)", c.args, id, yes, c.wantID, c.wantYes)
			}
		})
	}
}

func TestRenderTaskList(t *testing.T) {
	tasks := []task.Task{
		{ID: "2026-08-01-first", State: task.StateDraft, Agent: "claude"},
		{ID: "2026-08-05-second", State: task.StateImplementing},
	}
	titles := map[string]string{"2026-08-01-first": "add retries"}
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 4, 2, ' ', 0)
	renderTaskList(w, tasks, titles)
	_ = w.Flush()
	out := b.String()
	for _, want := range []string{"ID", "STATE", "AGENT", "TITLE", "2026-08-01-first", "draft", "claude", "add retries", "implementing"} {
		if !strings.Contains(out, want) {
			t.Errorf("task list missing %q:\n%s", want, out)
		}
	}
	// A task without an agent or title still renders a full row.
	if !strings.Contains(out, "2026-08-05-second") {
		t.Errorf("second task missing:\n%s", out)
	}
}

func TestShortSHA(t *testing.T) {
	if short("deadbeefcafe") != "deadbeef" {
		t.Errorf("short = %q", short("deadbeefcafe"))
	}
	if short("abc") != "abc" {
		t.Errorf("short(short) = %q", short("abc"))
	}
}
