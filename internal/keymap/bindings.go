package keymap

import (
	"strings"

	"github.com/drjzlyan/karya/internal/term"
)

// The canonical karya actions. One flat namespace; the leader chords that reach
// them are defined in DefaultBindings. See DESIGN.md §6.2.
const (
	ActionFocusLeft  ActionID = "pane.focus.left"
	ActionFocusDown  ActionID = "pane.focus.down"
	ActionFocusUp    ActionID = "pane.focus.up"
	ActionFocusRight ActionID = "pane.focus.right"

	ActionResizeLeft  ActionID = "pane.resize.left"
	ActionResizeDown  ActionID = "pane.resize.down"
	ActionResizeUp    ActionID = "pane.resize.up"
	ActionResizeRight ActionID = "pane.resize.right"

	ActionSplitRight ActionID = "pane.split.right"
	ActionSplitDown  ActionID = "pane.split.down"
	ActionEqualize   ActionID = "pane.equalize"
	ActionClosePane  ActionID = "pane.close"
	ActionZoomPane   ActionID = "pane.zoom"
	ActionSendLeader ActionID = "pane.send-leader"

	ActionTabNext ActionID = "tab.next"
	ActionTabPrev ActionID = "tab.prev"
	ActionTabNew  ActionID = "tab.new"
	ActionTabGoto ActionID = "tab.goto" // suffixed with the digit at dispatch time

	ActionPaneSwitcher ActionID = "pane.switcher"
	ActionFocusEditor  ActionID = "focus.editor"
	ActionFocusBuild   ActionID = "focus.build"

	ActionFindFile ActionID = "find.file"
	ActionSearch   ActionID = "search.project"

	ActionTaskBoard ActionID = "task.board"
	ActionTaskNew   ActionID = "task.new"
	ActionTaskStart ActionID = "task.start"

	ActionGitPanel  ActionID = "git.panel"
	ActionGitCommit ActionID = "git.commit"
	ActionGitPush   ActionID = "git.push"

	ActionReview     ActionID = "review.open"
	ActionAgentInbox ActionID = "agent.inbox"
	ActionHelpKeys   ActionID = "help.keys"
	ActionQuit       ActionID = "app.quit"

	// ActionTabGoto1..9 are the concrete per-digit tab jumps.
	ActionTabGoto1 ActionID = "tab.goto.1"
	ActionTabGoto2 ActionID = "tab.goto.2"
	ActionTabGoto3 ActionID = "tab.goto.3"
	ActionTabGoto4 ActionID = "tab.goto.4"
	ActionTabGoto5 ActionID = "tab.goto.5"
	ActionTabGoto6 ActionID = "tab.goto.6"
	ActionTabGoto7 ActionID = "tab.goto.7"
	ActionTabGoto8 ActionID = "tab.goto.8"
	ActionTabGoto9 ActionID = "tab.goto.9"
)

// Leader is karya's default leader key: Ctrl+Space. It can be overridden per
// user (e.g. on macOS, where Ctrl+Space is often grabbed by the OS input-source
// shortcut) via DefaultBindingsFor + ParseLeader.
var Leader = term.Ctrl(' ')

// ParseLeader interprets a leader spec like "ctrl+space", "ctrl+a", or "ctrl+\"
// into a Key. Unrecognized specs fall back to the default (Ctrl+Space).
func ParseLeader(spec string) term.Key {
	s := strings.ToLower(strings.TrimSpace(spec))
	s = strings.ReplaceAll(s, "c-", "ctrl+")
	switch s {
	case "", "ctrl+space", "ctrl+@":
		return term.Ctrl(' ')
	}
	if strings.HasPrefix(s, "ctrl+") {
		rest := strings.TrimPrefix(s, "ctrl+")
		if r := []rune(rest); len(r) == 1 {
			return term.Ctrl(r[0])
		}
	}
	return term.Ctrl(' ')
}

// DefaultBindings returns karya's unified binding table with the default leader.
func DefaultBindings() []Binding { return DefaultBindingsFor(Leader) }

// DefaultBindingsFor returns karya's unified binding table using the given leader
// key. Every IDE action is a <leader> chord; the same chords apply whatever pane
// has focus.
func DefaultBindingsFor(leader term.Key) []Binding {
	lead := func(runes ...rune) []term.Key {
		keys := make([]term.Key, 0, len(runes)+1)
		keys = append(keys, leader)
		for _, r := range runes {
			keys = append(keys, term.RuneKey(r))
		}
		return keys
	}
	const (
		gPanes = "Panes"
		gTabs  = "Tabs"
		gTasks = "Tasks"
		gGit   = "Git"
		gApp   = "karya"
	)
	b := []Binding{
		// Pane focus
		{Keys: lead('h'), Action: ActionFocusLeft, Desc: "Focus left", Group: gPanes},
		{Keys: lead('j'), Action: ActionFocusDown, Desc: "Focus down", Group: gPanes},
		{Keys: lead('k'), Action: ActionFocusUp, Desc: "Focus up", Group: gPanes},
		{Keys: lead('l'), Action: ActionFocusRight, Desc: "Focus right", Group: gPanes},
		// Pane resize
		{Keys: lead('H'), Action: ActionResizeLeft, Desc: "Resize left", Group: gPanes},
		{Keys: lead('J'), Action: ActionResizeDown, Desc: "Resize down", Group: gPanes},
		{Keys: lead('K'), Action: ActionResizeUp, Desc: "Resize up", Group: gPanes},
		{Keys: lead('L'), Action: ActionResizeRight, Desc: "Resize right", Group: gPanes},
		// Split / manage
		{Keys: lead('|'), Action: ActionSplitRight, Desc: "Split right", Group: gPanes},
		{Keys: lead('-'), Action: ActionSplitDown, Desc: "Split down", Group: gPanes},
		{Keys: lead('='), Action: ActionEqualize, Desc: "Equalize splits", Group: gPanes},
		{Keys: lead('x'), Action: ActionClosePane, Desc: "Close pane", Group: gPanes},
		{Keys: lead('z'), Action: ActionZoomPane, Desc: "Zoom pane", Group: gPanes},
		{Keys: lead('w'), Action: ActionPaneSwitcher, Desc: "Pane/window switcher", Group: gPanes},
		{Keys: lead('e'), Action: ActionFocusEditor, Desc: "Focus editor", Group: gPanes},
		{Keys: lead('b'), Action: ActionFocusBuild, Desc: "Build/test pane", Group: gPanes},
		// Find & search (human IDE features)
		{Keys: lead('f'), Action: ActionFindFile, Desc: "Find file", Group: gApp},
		{Keys: lead('/'), Action: ActionSearch, Desc: "Search project", Group: gApp},
		// Tabs
		{Keys: lead('c'), Action: ActionTabNew, Desc: "New tab", Group: gTabs},
		{Keys: lead('n'), Action: ActionTabNext, Desc: "Next tab", Group: gTabs},
		{Keys: lead('p'), Action: ActionTabPrev, Desc: "Previous tab", Group: gTabs},
		// Tasks group
		{Keys: lead('t', 't'), Action: ActionTaskBoard, Desc: "Task board", Group: gTasks},
		{Keys: lead('t', 'n'), Action: ActionTaskNew, Desc: "New task", Group: gTasks},
		{Keys: lead('t', 's'), Action: ActionTaskStart, Desc: "Start task", Group: gTasks},
		// Git group
		{Keys: lead('g', 'g'), Action: ActionGitPanel, Desc: "Git panel", Group: gGit},
		{Keys: lead('g', 'c'), Action: ActionGitCommit, Desc: "Commit", Group: gGit},
		{Keys: lead('g', 'p'), Action: ActionGitPush, Desc: "Push", Group: gGit},
		// Review / agent / help / quit
		{Keys: lead('r'), Action: ActionReview, Desc: "Review pending gate", Group: gApp},
		{Keys: lead('a'), Action: ActionAgentInbox, Desc: "Agent inbox / delegate", Group: gApp},
		{Keys: lead('?'), Action: ActionHelpKeys, Desc: "Keymap reference", Group: gApp},
		{Keys: lead('Q'), Action: ActionQuit, Desc: "Quit karya", Group: gApp},
		// Send a literal leader to the focused pane.
		{Keys: []term.Key{leader, leader}, Action: ActionSendLeader, Desc: "Send leader to pane", Group: gPanes},
	}
	// Tab jumps 1..9
	tabGoto := []ActionID{
		ActionTabGoto1, ActionTabGoto2, ActionTabGoto3, ActionTabGoto4, ActionTabGoto5,
		ActionTabGoto6, ActionTabGoto7, ActionTabGoto8, ActionTabGoto9,
	}
	for i := 0; i < 9; i++ {
		b = append(b, Binding{
			Keys:   lead(rune('1' + i)),
			Action: tabGoto[i],
			Desc:   "Go to tab " + string(rune('1'+i)),
			Group:  gTabs,
		})
	}
	return b
}
