package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/drjzlyan/karya/internal/tutorial"
)

func TestSelectLessons(t *testing.T) {
	all := tutorial.Lessons()

	got, err := selectLessons(nil)
	if err != nil || len(got) != len(all) {
		t.Errorf("selectLessons(nil) = %d lessons, %v; want all (%d)", len(got), err, len(all))
	}

	got, err = selectLessons([]string{"2"})
	if err != nil || len(got) != 1 || got[0].Num != 2 {
		t.Errorf("selectLessons([2]) = %v, %v; want lesson 2", got, err)
	}

	for _, bad := range [][]string{{"0"}, {"999"}, {"abc"}} {
		if _, err := selectLessons(bad); err == nil {
			t.Errorf("selectLessons(%v) expected an error", bad)
		}
	}
}

func TestListLessons(t *testing.T) {
	var buf bytes.Buffer
	listLessons(&buf)
	out := buf.String()
	for _, l := range tutorial.Lessons() {
		if !strings.Contains(out, l.Title) {
			t.Errorf("listLessons missing %q", l.Title)
		}
	}
	if !strings.Contains(out, "karya tutorial ide") {
		t.Errorf("listLessons should mention the IDE tutorial:\n%s", out)
	}
}

// TestRunTutorialInteractiveTyping drives the walkthrough with scripted input: a
// mistyped command must re-prompt, then the correct command advances and the
// sandbox verification runs. Uses the first actionable (Expect) lesson.
func TestRunTutorialInteractiveTyping(t *testing.T) {
	sb, err := tutorial.NewSandbox()
	if err != nil {
		t.Fatalf("NewSandbox: %v", err)
	}
	t.Cleanup(func() { _ = sb.Cleanup() })

	var lesson tutorial.Lesson
	for _, l := range tutorial.Lessons() {
		if l.Expect != "" && l.Run != nil {
			lesson = l
			break
		}
	}
	if lesson.Expect == "" {
		t.Fatal("no actionable lesson with an Expect command found")
	}

	// A wrong line first (should re-prompt), then the correct command.
	in := strings.NewReader("wrong command\n" + lesson.Expect + "\n")
	var out bytes.Buffer
	code := runTutorial(in, &out, true, sb, []tutorial.Lesson{lesson})
	got := out.String()

	if code != 0 {
		t.Errorf("runTutorial exit = %d, want 0:\n%s", code, got)
	}
	if !strings.Contains(got, "Now try it — type:") {
		t.Errorf("expected a typing prompt:\n%s", got)
	}
	if !strings.Contains(got, "Not quite") {
		t.Errorf("wrong input should re-prompt with a hint:\n%s", got)
	}
	if !strings.ContainsAny(got, "✓•") {
		t.Errorf("correct input should run the sandbox verification (✓/•):\n%s", got)
	}
}

// TestRunTutorialNonInteractive confirms piped runs never block on input and
// still verify every lesson (backward-compatible with scripts and CI).
func TestRunTutorialNonInteractive(t *testing.T) {
	sb, err := tutorial.NewSandbox()
	if err != nil {
		t.Fatalf("NewSandbox: %v", err)
	}
	t.Cleanup(func() { _ = sb.Cleanup() })

	var out bytes.Buffer
	code := runTutorial(strings.NewReader(""), &out, false, sb, tutorial.Lessons())
	got := out.String()
	if code != 0 {
		t.Errorf("runTutorial exit = %d, want 0:\n%s", code, got)
	}
	if strings.Contains(got, "Now try it — type:") {
		t.Errorf("non-interactive run should not prompt for typing:\n%s", got)
	}
}
