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
	log    string // canned `git log` output (pretty %h\x1f%s\x1f%an\x1f%cr)
	runs   [][]string
	shown  []string // refs passed to `git show`
}

func (f *fakeRunner) Output(dir, name string, args ...string) (string, error) {
	switch {
	case len(args) > 0 && args[0] == "status":
		return f.status, nil
	case len(args) > 1 && args[0] == "rev-parse" && args[1] == "--abbrev-ref":
		return "main", nil
	case len(args) > 0 && args[0] == "diff":
		return "diff --git a/x b/x\n@@ -0,0 +1 @@\n+hi\n", nil
	case len(args) > 0 && args[0] == "log":
		return f.log, nil
	case len(args) > 0 && args[0] == "show":
		f.shown = append(f.shown, args[len(args)-1])
		return "commit diff\n@@ -0,0 +1 @@\n+x\n", nil
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

// sampleLog is two commits in git's pretty format (hash, subject, author, when).
const sampleLog = "abc123\x1ffirst change\x1fAda\x1f2 hours ago\ndef456\x1fsecond change\x1fLin\x1f3 days ago"

func newPanelWithLog(status, log string) (*Panel, *fakeRunner) {
	fr := &fakeRunner{status: status, log: log}
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

func TestPanelLoadsLog(t *testing.T) {
	p, _ := newPanelWithLog(" M a.go", sampleLog)
	if len(p.commits) != 2 {
		t.Fatalf("want 2 commits, got %d", len(p.commits))
	}
	if p.commits[0].Subject != "first change" || p.commits[0].When != "2 hours ago" {
		t.Fatalf("commit not parsed: %+v", p.commits[0])
	}
}

func TestPanelCleanTreeFocusesLog(t *testing.T) {
	// A clean tree has nothing to stage, so the panel should default to the log
	// and preview the tip commit's diff — no empty void.
	p, fr := newPanelWithLog("", sampleLog)
	if p.focus != focusLog {
		t.Fatalf("clean tree should focus the log")
	}
	if len(fr.shown) == 0 || fr.shown[0] != "abc123" {
		t.Fatalf("should have shown the tip commit; shown=%v", fr.shown)
	}
	if len(p.diff) == 0 {
		t.Fatalf("commit diff should be loaded on a clean tree")
	}
}

func TestPanelTabTogglesFocus(t *testing.T) {
	p, _ := newPanelWithLog(" M a.go", sampleLog) // dirty → starts on changes
	if p.focus != focusChanges {
		t.Fatalf("dirty tree should start on changes")
	}
	p.HandleKey(term.Named(term.SymTab))
	if p.focus != focusLog {
		t.Fatalf("Tab should switch to log")
	}
	p.HandleKey(term.Named(term.SymTab))
	if p.focus != focusChanges {
		t.Fatalf("Tab should switch back to changes")
	}
}

func TestPanelLogNavigationShowsCommit(t *testing.T) {
	p, fr := newPanelWithLog(" M a.go", sampleLog)
	p.HandleKey(term.Named(term.SymTab)) // focus log (shows abc123)
	p.HandleKey(term.RuneKey('j'))       // move to second commit
	if p.logSel != 1 {
		t.Fatalf("logSel = %d want 1", p.logSel)
	}
	last := fr.shown[len(fr.shown)-1]
	if last != "def456" {
		t.Fatalf("moving in the log should show that commit; last shown=%q", last)
	}
	// j/k must drive the log, not the file list, while the log is focused.
	if p.sel != 0 {
		t.Fatalf("file selection must not move while log is focused; sel=%d", p.sel)
	}
}

func TestPanelViewShowsLogSection(t *testing.T) {
	p, _ := newPanelWithLog("", sampleLog)
	buf := cellbuf.New(80, 16)
	p.View(buf, cellbuf.Rect{X: 0, Y: 0, W: 80, H: 16}, true)
	out := buf.String()
	for _, want := range []string{"Log (2)", "abc123", "first change"} {
		if !strings.Contains(out, want) {
			t.Fatalf("log section missing %q:\n%s", want, out)
		}
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
