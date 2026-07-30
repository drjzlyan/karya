package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/drjzlyan/karya/internal/doctor"
)

func TestRenderReport(t *testing.T) {
	r := doctor.Report{Checks: []doctor.Check{
		{Group: "tools", Name: "tmux", Level: doctor.OK, Detail: "found (3.4)"},
		{Group: "tools", Name: "git", Level: doctor.Warn, Detail: "not found"},
		{Group: "agents", Name: "coding agent", Level: doctor.Problem, Detail: "none"},
	}}
	var buf bytes.Buffer
	renderReport(&buf, r)
	out := buf.String()

	for _, want := range []string{"tools", "agents", "tmux", "✓", "•", "✗",
		"1 ok · 1 warning(s) · 1 problem(s)", "Some required checks failed"} {
		if !strings.Contains(out, want) {
			t.Errorf("renderReport output missing %q; got:\n%s", want, out)
		}
	}
}

func TestMarker(t *testing.T) {
	cases := map[doctor.Level]string{doctor.OK: "✓", doctor.Warn: "•", doctor.Problem: "✗"}
	for level, want := range cases {
		if got := marker(level); got != want {
			t.Errorf("marker(%v) = %q, want %q", level, got, want)
		}
	}
}
