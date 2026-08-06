package gitui

import (
	"strings"
	"testing"

	"github.com/drjzlyan/karya/internal/cellbuf"
	"github.com/drjzlyan/karya/internal/git"
	"github.com/drjzlyan/karya/internal/term"
)

// fakeRunner returns canned git output and records Run calls.
type fakeRunner struct {
	status string
	runs   [][]string
}

func (f *fakeRunner) Output(dir, name string, args ...string) (string, error) {
	switch {
	case len(args) > 0 && args[0] == "status":
		return f.status, nil
	case len(args) > 1 && args[0] == "rev-parse" && args[1] == "--abbrev-ref":
		return "main", nil
	case len(args) > 0 && args[0] == "diff":
		return "diff --git a/x b/x\n@@ -0,0 +1 @@\n+hi\n", nil
	}
	return "", nil
}

func (f *fakeRunner) Run(dir, name string, args ...string) error {
	f.runs = append(f.runs, args)
	return nil
}

func (f *fakeRunner) ran(sub string) bool {
	for _, r := range f.runs {
		if strings.Contains(strings.Join(r, " "), sub) {
			return true
		}
	}
	return false
}

func newPanel(status string) (*Panel, *fakeRunner) {
	fr := &fakeRunner{status: status}
	return New(git.New("/repo", fr)), fr
}

func TestPanelLoadsStatus(t *testing.T) {
	p, _ := newPanel(" M a.go\n?? b.txt")
	if p.branch != "main" {
		t.Fatalf("branch = %q", p.branch)
	}
	if len(p.files) != 2 {
		t.Fatalf("want 2 files, got %d", len(p.files))
	}
}

func TestPanelToggleStage(t *testing.T) {
	p, fr := newPanel(" M a.go") // unstaged
	p.HandleKey(term.RuneKey(' '))
	if !fr.ran("add -- a.go") {
		t.Fatalf("space should stage the file; runs=%v", fr.runs)
	}
}

func TestPanelToggleUnstage(t *testing.T) {
	p, fr := newPanel("M  a.go") // staged only
	p.HandleKey(term.RuneKey(' '))
	if !fr.ran("restore --staged") && !fr.ran("reset") {
		t.Fatalf("space should unstage a staged file; runs=%v", fr.runs)
	}
}

func TestPanelStageAll(t *testing.T) {
	p, fr := newPanel(" M a.go\n?? b.txt")
	p.HandleKey(term.RuneKey('a'))
	if !fr.ran("add -A") {
		t.Fatalf("'a' should stage all; runs=%v", fr.runs)
	}
}

func TestPanelCommitFlow(t *testing.T) {
	p, fr := newPanel("M  a.go")
	p.HandleKey(term.RuneKey('c')) // enter commit mode
	if p.mode != modeCommit {
		t.Fatalf("should be in commit mode")
	}
	for _, r := range "hi" {
		p.HandleKey(term.RuneKey(r))
	}
	if p.commitBuf != "hi" {
		t.Fatalf("commit buffer = %q", p.commitBuf)
	}
	p.HandleKey(term.Named(term.SymEnter))
	if !fr.ran("commit -m hi") {
		t.Fatalf("commit not run; runs=%v", fr.runs)
	}
	if p.mode != modeNormal {
		t.Fatalf("should return to normal mode after commit")
	}
}

func TestPanelCommitEscCancels(t *testing.T) {
	p, fr := newPanel("M  a.go")
	p.HandleKey(term.RuneKey('c'))
	p.HandleKey(term.RuneKey('x'))
	p.HandleKey(term.Named(term.SymEsc))
	if p.mode != modeNormal || p.commitBuf != "" {
		t.Fatalf("esc should cancel commit input")
	}
	if fr.ran("commit") {
		t.Fatalf("esc should not commit")
	}
}

func TestPanelQuitCloses(t *testing.T) {
	p, _ := newPanel(" M a.go")
	if p.Done() {
		t.Fatal("panel should start open")
	}
	p.HandleKey(term.RuneKey('q'))
	if !p.Done() {
		t.Fatal("q should close the panel")
	}
}

func TestPanelMoveSelection(t *testing.T) {
	p, _ := newPanel(" M a.go\n M b.go\n M c.go")
	p.HandleKey(term.RuneKey('j'))
	p.HandleKey(term.RuneKey('j'))
	if p.sel != 2 {
		t.Fatalf("sel = %d want 2", p.sel)
	}
	p.HandleKey(term.RuneKey('j')) // clamp at end
	if p.sel != 2 {
		t.Fatalf("sel should clamp at 2, got %d", p.sel)
	}
	p.HandleKey(term.RuneKey('k'))
	if p.sel != 1 {
		t.Fatalf("sel = %d want 1", p.sel)
	}
}

func TestPanelViewRenders(t *testing.T) {
	p, _ := newPanel(" M a.go\n?? b.txt")
	buf := cellbuf.New(60, 12)
	p.View(buf, cellbuf.Rect{X: 0, Y: 0, W: 60, H: 12}, true)
	out := buf.String()
	if !strings.Contains(out, "branch: main") {
		t.Fatalf("header missing:\n%s", out)
	}
	if !strings.Contains(out, "a.go") || !strings.Contains(out, "b.txt") {
		t.Fatalf("file list missing:\n%s", out)
	}
	if !strings.Contains(out, "commit") {
		t.Fatalf("help/status line missing:\n%s", out)
	}
}

func TestPanelViewCommitMode(t *testing.T) {
	p, _ := newPanel("M  a.go")
	p.EnterCommit()
	p.commitBuf = "wip"
	buf := cellbuf.New(60, 12)
	p.View(buf, cellbuf.Rect{X: 0, Y: 0, W: 60, H: 12}, true)
	if !strings.Contains(buf.String(), "commit: wip") {
		t.Fatalf("commit input not shown:\n%s", buf.String())
	}
}
