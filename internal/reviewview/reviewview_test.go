package reviewview

import (
	"strings"
	"testing"

	"github.com/drjzlyan/karya/internal/cellbuf"
	"github.com/drjzlyan/karya/internal/gate"
	"github.com/drjzlyan/karya/internal/review"
	"github.com/drjzlyan/karya/internal/spec"
	"github.com/drjzlyan/karya/internal/task"
	"github.com/drjzlyan/karya/internal/term"
)

func sampleReview() *review.Review {
	return &review.Review{
		Task: task.Task{ID: "2026-08-06-retry", State: task.StatePlanned},
		Spec: &spec.Spec{
			Objective: "Retry transient download failures.",
			Criteria: []spec.Criterion{
				{Text: "5xx retried 3x", Checked: true},
				{Text: "4xx never retried", Checked: false},
			},
		},
		Plan:     "1. add backoff\n2. cap retries",
		Diff:     "diff --git a/x b/x\n@@ -1 +1 @@\n-old\n+new\n",
		Evidence: []string{"go test ./... ok"},
		Pending:  gate.Pending{Gate: task.GatePlan, Approve: task.StateApproved, Reject: task.StateDraft},
		HasGate:  true,
	}
}

func TestReviewViewRenders(t *testing.T) {
	p := New(sampleReview())
	buf := cellbuf.New(80, 30)
	p.View(buf, cellbuf.Rect{X: 0, Y: 0, W: 80, H: 30}, true)
	out := buf.String()
	for _, want := range []string{
		"Task 2026-08-06-retry",
		"Gate: plan",
		"Objective",
		"Retry transient download failures.",
		"Acceptance criteria",
		"[x] 5xx retried 3x",
		"[ ] 4xx never retried",
		"Plan",
		"add backoff",
		"Diff",
		"Verification 1",
		"karya gate approve 2026-08-06-retry",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("review render missing %q:\n%s", want, out)
		}
	}
}

func TestReviewViewScrollAndClose(t *testing.T) {
	p := New(sampleReview())
	if p.Done() {
		t.Fatal("should start open")
	}
	before := p.scroll
	p.HandleKey(term.RuneKey('j'))
	if p.scroll != before+1 {
		t.Fatalf("j should scroll down")
	}
	p.HandleKey(term.RuneKey('k'))
	if p.scroll != before {
		t.Fatalf("k should scroll up")
	}
	p.HandleKey(term.RuneKey('q'))
	if !p.Done() {
		t.Fatal("q should close")
	}
}

func TestReviewViewDiffColored(t *testing.T) {
	p := New(sampleReview())
	buf := cellbuf.New(80, 40)
	p.View(buf, cellbuf.Rect{X: 0, Y: 0, W: 80, H: 40}, true)
	// Find the "+new" line and confirm it's green.
	found := false
	for y := 0; y < 40; y++ {
		if buf.Cell(0, y).Rune == '+' && buf.Cell(1, y).Rune == 'n' {
			if buf.Cell(0, y).Style.FG == cellbuf.Palette(2) {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("added diff line not rendered green:\n%s", buf.String())
	}
}
