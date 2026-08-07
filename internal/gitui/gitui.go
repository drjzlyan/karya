// Package gitui is karya's built-in git panel — the interactive surface that
// replaces lazygit (DESIGN.md §6). It is a thin view over the headless
// internal/git service: a file list (staged/unstaged), a live diff of the
// selected file, and stage/unstage/commit/push actions. All git logic lives in
// internal/git, so this package is about presentation + key handling and is
// snapshot/model tested.
package gitui

import (
	"github.com/drjzlyan/karya/internal/diffview"
	"github.com/drjzlyan/karya/internal/git"
	"github.com/drjzlyan/karya/internal/term"
)

type mode uint8

const (
	modeNormal mode = iota
	modeCommit
)

// focusArea is which list the panel's motion keys drive.
type focusArea uint8

const (
	focusChanges focusArea = iota // the working-tree file list
	focusLog                      // the commit history
)

// logDepth is how many recent commits the panel loads, so the history is useful
// (and scrollable) even on a clean tree.
const logDepth = 50

// Panel is the git panel view.
type Panel struct {
	repo *git.Repo

	branch string
	files  []git.FileStatus
	sel    int

	commits []git.Commit
	logSel  int

	focus      focusArea
	diff       []diffview.Line
	diffScroll int

	mode      mode
	commitBuf string
	status    string
	closed    bool
}

// New builds a panel for repo and loads the initial status.
func New(repo *git.Repo) *Panel {
	p := &Panel{repo: repo}
	p.refresh()
	return p
}

// Done reports whether the panel asked to close.
func (p *Panel) Done() bool { return p.closed }

// EnterCommit switches the panel into commit-message input mode.
func (p *Panel) EnterCommit() {
	p.mode = modeCommit
	p.commitBuf = ""
}

// Push pushes the current branch and records the result in the status line.
func (p *Panel) Push() {
	if err := p.repo.Push(); err != nil {
		p.status = "push failed: " + err.Error()
		return
	}
	p.status = "pushed " + p.branch
}

func (p *Panel) refresh() {
	if b, err := p.repo.CurrentBranch(); err == nil {
		p.branch = b
	}
	files, err := p.repo.Status()
	if err != nil {
		p.status = err.Error()
	}
	p.files = files
	if p.sel >= len(files) {
		p.sel = max(0, len(files)-1)
	}
	if commits, err := p.repo.Log(logDepth); err == nil {
		p.commits = commits
	}
	if p.logSel >= len(p.commits) {
		p.logSel = max(0, len(p.commits)-1)
	}
	// On a clean tree there is nothing to stage, so default motion to history —
	// the panel stays useful instead of showing an empty void.
	if len(p.files) == 0 && len(p.commits) > 0 {
		p.focus = focusLog
	}
	p.loadDiff()
}

func (p *Panel) loadDiff() {
	p.diffScroll = 0
	if p.focus == focusLog {
		if len(p.commits) == 0 {
			p.diff = nil
			return
		}
		d, _ := p.repo.Show(p.commits[p.logSel].Hash)
		p.diff = diffview.Parse(d)
		return
	}
	if len(p.files) == 0 {
		p.diff = nil
		return
	}
	f := p.files[p.sel]
	// Prefer the staged diff for a purely-staged file; otherwise the worktree diff.
	staged := f.Staged() && !f.Unstaged()
	d, _ := p.repo.Diff(f.Path, staged)
	p.diff = diffview.Parse(d)
}

// HandleKey processes a key forwarded to the focused panel.
func (p *Panel) HandleKey(k term.Key) {
	if p.mode == modeCommit {
		p.handleCommitKey(k)
		return
	}
	switch {
	case k == term.RuneKey('j') || k == term.Named(term.SymDown):
		p.move(1)
	case k == term.RuneKey('k') || k == term.Named(term.SymUp):
		p.move(-1)
	case k == term.Named(term.SymTab):
		p.toggleFocus()
	case k == term.RuneKey(' ') || k == term.Named(term.SymEnter):
		p.toggleStage()
	case k == term.RuneKey('a'):
		_ = p.repo.StageAll()
		p.refresh()
	case k == term.RuneKey('u'):
		_ = p.repo.UnstageAll()
		p.refresh()
	case k == term.RuneKey('c'):
		p.EnterCommit()
	case k == term.RuneKey('P'):
		p.Push()
	case k == term.RuneKey('r'):
		p.refresh()
	case k == term.Ctrl('d'):
		p.diffScroll = min(p.diffScroll+5, diffview.MaxScroll(p.diff, 1))
	case k == term.Ctrl('u'):
		p.diffScroll = max(p.diffScroll-5, 0)
	case k == term.RuneKey('q') || k == term.Named(term.SymEsc):
		p.closed = true
	}
}

func (p *Panel) handleCommitKey(k term.Key) {
	switch {
	case k == term.Named(term.SymEsc):
		p.mode = modeNormal
		p.commitBuf = ""
	case k == term.Named(term.SymEnter):
		msg := p.commitBuf
		p.mode = modeNormal
		p.commitBuf = ""
		if msg == "" {
			p.status = "commit aborted: empty message"
			return
		}
		if err := p.repo.Commit(msg); err != nil {
			p.status = "commit failed: " + err.Error()
		} else {
			p.status = "committed: " + msg
		}
		p.refresh()
	case k == term.Named(term.SymBackspace):
		if n := len(p.commitBuf); n > 0 {
			p.commitBuf = p.commitBuf[:n-1]
		}
	case k.Sym == term.SymRune && k.Mod == 0:
		p.commitBuf += string(k.Rune)
	}
}

// toggleFocus switches motion keys between the file list and the commit log,
// then reloads the diff for whatever the newly focused list has selected.
func (p *Panel) toggleFocus() {
	if p.focus == focusChanges {
		p.focus = focusLog
	} else {
		p.focus = focusChanges
	}
	p.loadDiff()
}

func (p *Panel) move(delta int) {
	if p.focus == focusLog {
		if len(p.commits) == 0 {
			return
		}
		p.logSel = clamp(p.logSel+delta, 0, len(p.commits)-1)
		p.loadDiff()
		return
	}
	if len(p.files) == 0 {
		return
	}
	p.sel = clamp(p.sel+delta, 0, len(p.files)-1)
	p.loadDiff()
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (p *Panel) toggleStage() {
	if len(p.files) == 0 {
		return
	}
	f := p.files[p.sel]
	if f.Staged() && !f.Unstaged() {
		_ = p.repo.Unstage(f.Path)
	} else {
		_ = p.repo.Stage(f.Path)
	}
	p.refresh()
}
