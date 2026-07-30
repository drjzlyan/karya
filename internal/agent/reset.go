package agent

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/drjzlyan/karya/internal/config"
)

// Reset restores the default dev-window layout: the editor pane on the left and
// the agent / build-test panes stacked on the right, then relaunches the current
// agent. If the whole dev window has been lost it is recreated from scratch.
// karya's isolated env means panes inherit NVIM_APPNAME/EDITOR from the session
// environment, so no per-pane exports are needed.
func (m *Manager) Reset() error {
	workdir := m.Workdir()
	if workdir == "" {
		workdir = home()
	}

	if !m.hasDevWindow() {
		return m.recreateDevWindow(workdir)
	}

	// Kill every pane except the editor (pane 1), highest index first so the
	// indices below it stay stable.
	for _, idx := range m.paneIndexesDesc() {
		if idx != "1" {
			_ = m.tmux.Run("kill-pane", "-t", m.session+":dev."+idx)
		}
	}
	m.buildRightColumn(workdir)
	m.relaunchCurrent()
	_ = m.tmux.Run("select-pane", "-t", m.session+":dev.1")
	m.notify("Layout reset")
	return nil
}

// recreateDevWindow rebuilds the dev window in full when it has been closed.
func (m *Manager) recreateDevWindow(workdir string) error {
	m.notify(fmt.Sprintf("Recreating 'dev' window in session '%s'", m.session))
	if err := m.tmux.Run("new-window", "-t", m.session, "-n", "dev", "-c", workdir); err != nil {
		return fmt.Errorf("recreate dev window: %w", err)
	}
	_ = m.tmux.Run("select-pane", "-t", m.session+":dev.1", "-T", "editor")
	_ = m.tmux.Run("send-keys", "-t", m.session+":dev.1", "NVIM_APPNAME="+config.NvimAppName+" nvim", "Enter")
	m.buildRightColumn(workdir)
	m.relaunchCurrent()
	_ = m.tmux.Run("select-pane", "-t", m.session+":dev.1")
	m.notify("Layout reset")
	return nil
}

// buildRightColumn splits the editor pane into the agent (top) and build/test
// (bottom) panes and records their pane ids, matching session.Build.
func (m *Manager) buildRightColumn(workdir string) {
	p1 := m.session + ":dev.1"
	p2 := m.session + ":dev.2"
	p3 := m.session + ":dev.3"
	_ = m.tmux.Run("select-pane", "-t", p1, "-T", "editor")
	_ = m.tmux.Run("split-window", "-h", "-l", "35%", "-t", p1, "-c", workdir)
	_ = m.tmux.Run("split-window", "-v", "-l", "40%", "-t", p2, "-c", workdir)
	_ = m.tmux.Run("select-pane", "-t", p2, "-T", "agent")
	_ = m.tmux.Run("select-pane", "-t", p3, "-T", "build/test")

	m.setOpt("@ide_agent_pane", m.paneID(p2))
	m.setOpt("@ide_editor_pane", m.paneID(p1))
	m.setOpt("@ide_shell_pane", m.paneID(p3))
}

// relaunchCurrent starts the current agent in the freshly built agent pane.
func (m *Manager) relaunchCurrent() {
	cur := m.Current()
	if cur != None && Available(cur) {
		time.Sleep(time.Second) // let the shell settle before typing
		_ = m.tmux.Run("send-keys", "-t", m.session+":dev.2", cur, "Enter")
	}
}

func (m *Manager) hasDevWindow() bool {
	out, err := m.tmux.Output("list-windows", "-t", m.session, "-F", "#{window_name}")
	if err != nil {
		return false
	}
	for _, w := range strings.Fields(out) {
		if w == "dev" {
			return true
		}
	}
	return false
}

// paneIndexesDesc returns the dev window's pane indexes in descending order.
func (m *Manager) paneIndexesDesc() []string {
	out, err := m.tmux.Output("list-panes", "-t", m.session+":dev", "-F", "#{pane_index}")
	if err != nil {
		return nil
	}
	idxs := strings.Fields(out)
	// Descending so killing a pane never renumbers one we have yet to visit.
	for i, j := 0, len(idxs)-1; i < j; i, j = i+1, j-1 {
		idxs[i], idxs[j] = idxs[j], idxs[i]
	}
	return idxs
}

func (m *Manager) paneID(target string) string {
	id, err := m.tmux.Output("display-message", "-p", "-t", target, "#{pane_id}")
	if err != nil {
		return ""
	}
	return id
}

// StatusText returns a human-readable status block for `karya agent status` when
// run inside a session: the session, current agent, available agents, and the
// saved per-project preference.
func (m *Manager) StatusText() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Session: %s\n", m.session)
	fmt.Fprintf(&b, "Current agent: %s\n", m.Current())
	agents := m.Agents()
	if len(agents) == 0 {
		fmt.Fprintln(&b, "Available agents: none detected")
	} else {
		fmt.Fprintf(&b, "Available agents: %s\n", strings.Join(agents, ", "))
	}
	if workdir := m.Workdir(); workdir != "" {
		saved := m.prefs.Get("agent." + workdir)
		if saved == "" {
			saved = "none"
		}
		fmt.Fprintf(&b, "Saved preference: %s", saved)
	}
	return strings.TrimRight(b.String(), "\n")
}

// ClearPref removes the saved agent preference for this session's workdir.
func (m *Manager) ClearPref() error {
	workdir := m.Workdir()
	if workdir == "" {
		m.notify("No workdir stored for this session")
		return nil
	}
	if err := m.prefs.Delete("agent." + workdir); err != nil {
		return err
	}
	m.notify("Cleared agent preference for " + workdir)
	return nil
}

func home() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return "."
}
