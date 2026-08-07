package ide

import (
	"strings"
	"testing"

	"github.com/drjzlyan/karya/internal/taskview"
	"github.com/drjzlyan/karya/internal/term"
)

// TestLeaderNewTaskOpensBoardInInput checks that `<L> t n` opens the board and
// forwards a typed slug into its new-task input (not the layout keymap).
func TestLeaderNewTaskOpensBoardInInput(t *testing.T) {
	m := testModel(80, 24)
	press(m, leaderThen('t', 'n')...)
	if _, ok := m.tree.FocusedContent().(*taskview.Board); !ok {
		t.Fatalf("focused content = %T, want *taskview.Board", m.tree.FocusedContent())
	}
	// In input mode, 'x'/'y' build the slug (they are NOT board navigation), and
	// Enter submits — forward() consumes the request and returns the background
	// lifecycle command. If input mode had not been entered, 'x'/'y' would be
	// no-ops and Enter would open a review (a nil command), so a non-nil command
	// here proves the whole `<L> t n` → type → create path.
	press(m, term.RuneKey('x'), term.RuneKey('y'))
	cmd := press(m, term.Named(term.SymEnter))
	if cmd == nil {
		t.Fatal("submitting the slug should emit a background lifecycle command")
	}
}

func TestFirstLine(t *testing.T) {
	if got := firstLine("\n\n  boom: it broke  \nmore\n"); got != "boom: it broke" {
		t.Errorf("firstLine = %q", got)
	}
	if got := firstLine("   \n\t\n"); got != "" {
		t.Errorf("firstLine of blank = %q want empty", got)
	}
}

// TestLifecycleDoneRefreshesBoard checks that a completed lifecycle step reloads
// the board and surfaces the result, without opening the editor for non-new ops.
func TestLifecycleDoneRefreshesBoard(t *testing.T) {
	m := testModel(80, 24)
	loads := 0
	m.workspaces[WSAgents].board = taskview.New(func() []taskview.Item {
		loads++
		return []taskview.Item{{ID: "t1", State: "planned", Title: "x"}}
	})
	before := loads
	cmd := m.handleLifecycleDone(lifecycleDoneMsg{
		req: taskview.LifecycleRequest{Op: "plan", ID: "t1"},
	})
	if loads != before+1 {
		t.Fatalf("board not refreshed: loads=%d want %d", loads, before+1)
	}
	if cmd != nil {
		t.Fatal("non-new op should not return a command")
	}
	if !strings.Contains(m.status, "plan t1") {
		t.Fatalf("status = %q, want it to mention the step", m.status)
	}
}
