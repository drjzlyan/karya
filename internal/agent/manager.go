package agent

import (
	"fmt"
	"strings"
	"time"
)

// TmuxRunner is the narrow slice of tmux operations agent management needs. It
// is defined here (by the consumer) so the Manager depends on an abstraction,
// not on *tmuxx.Tmux — which keeps unit tests hermetic and satisfies the
// dependency-inversion rule in AGENT.md. *tmuxx.Tmux implements it.
type TmuxRunner interface {
	Run(args ...string) error
	Output(args ...string) (string, error)
}

// PrefStore persists per-project agent preferences. Defined by the consumer so
// the Manager needs only these methods; *prefs.Store implements it.
type PrefStore interface {
	Get(key string) string
	Set(key, value string) error
	Delete(key string) error
}

// Manager performs in-session agent operations (switch/next/prev/reset/status)
// against a single tmux session, reading and writing the @ide_* session options
// that session.Build seeds.
type Manager struct {
	tmux    TmuxRunner
	prefs   PrefStore
	session string          // tmux session name
	bin     string          // absolute karya binary, for command-prompt callbacks
	detect  func() []string // agent detector; overridable in tests
}

// NewManager binds a Manager to a tmux session. bin is used only by the
// interactive switcher to build the command-prompt callback.
func NewManager(t TmuxRunner, p PrefStore, session, bin string) *Manager {
	return &Manager{tmux: t, prefs: p, session: session, bin: bin, detect: Detect}
}

// Agents returns the session's available agents (@ide_agents), detecting and
// persisting them if the option is unset — mirroring ensure_agents.
func (m *Manager) Agents() []string {
	agents := m.getOpt("@ide_agents")
	if strings.TrimSpace(agents) == "" {
		agents = strings.Join(m.detect(), " ")
		m.setOpt("@ide_agents", agents)
	}
	return strings.Fields(agents)
}

// Current returns the active agent (@ide_current_agent), defaulting to None and
// persisting that default when unset — mirroring ensure_current.
func (m *Manager) Current() string {
	cur := strings.TrimSpace(m.getOpt("@ide_current_agent"))
	if cur == "" {
		cur = None
		m.setOpt("@ide_current_agent", cur)
	}
	return cur
}

// Workdir returns the session's working directory (@ide_workdir).
func (m *Manager) Workdir() string { return m.getOpt("@ide_workdir") }

// Next cycles to the following available agent and launches it.
func (m *Manager) Next() error { return m.cycle(true) }

// Prev cycles to the preceding available agent and launches it.
func (m *Manager) Prev() error { return m.cycle(false) }

// cycle moves one step through Agents() from Current() and launches the result.
func (m *Manager) cycle(forward bool) error {
	agents := m.Agents()
	if len(agents) == 0 {
		m.notify("No agents available")
		return nil
	}
	idx := cycleIndex(len(agents), indexOf(agents, m.Current()), forward)
	next := agents[idx]
	if err := m.launch(next); err != nil {
		return err
	}
	m.notify("Agent: " + next)
	return nil
}

// SwitchTo launches a specific agent by name (or None), validating that it is on
// PATH. Called from the interactive command-prompt callback and the CLI.
func (m *Manager) SwitchTo(name string) error {
	if name != None && !Available(name) {
		m.notify(fmt.Sprintf("Agent %q not found", name))
		return nil
	}
	if err := m.launch(name); err != nil {
		return err
	}
	m.notify("Agent: " + name)
	return nil
}

// SwitchInteractive opens a tmux command-prompt listing the available agents and
// routes the answer back through `karya agent switch-to <name>`.
func (m *Manager) SwitchInteractive() error {
	agents := m.Agents()
	if len(agents) == 0 {
		m.notify("No agents detected")
		return nil
	}
	prompt := fmt.Sprintf("Switch agent [%s] (current: %s, or 'none'):",
		strings.Join(agents, " "), m.Current())
	// %% is tmux's command-prompt placeholder for the typed reply.
	callback := "run-shell '" + m.bin + " agent switch-to %%'"
	return m.tmux.Run("command-prompt", "-p", prompt, callback)
}

// launch respawns the agent pane and starts the named agent in it, updating
// @ide_current_agent and saving the per-project preference. If the agent pane
// has disappeared it resets the layout first.
func (m *Manager) launch(name string) error {
	workdir := m.Workdir()
	if workdir == "" {
		workdir = home()
	}
	pane := m.getOpt("@ide_agent_pane")
	if !m.paneExists(pane) {
		if err := m.Reset(); err != nil {
			return err
		}
		pane = m.getOpt("@ide_agent_pane")
	}

	// Respawn kills any process in the pane and opens a fresh shell.
	_ = m.tmux.Run("respawn-pane", "-t", pane, "-k", "-c", workdir)
	time.Sleep(300 * time.Millisecond)

	// Start the agent through its Runner so the launch path is engine-agnostic
	// (the native engine plugs in behind the same interface, ROADMAP Phase 13).
	if cmd, ok := NewRunner(name).InteractiveCommand(); ok && Available(name) {
		_ = m.tmux.Run("send-keys", "-t", pane, cmd, "Enter")
	}

	m.setOpt("@ide_current_agent", name)
	m.savePref(workdir, name)
	_ = m.tmux.Run("select-pane", "-t", pane, "-T", "agent")
	_ = m.tmux.Run("select-pane", "-t", m.session+":dev.1") // focus editor
	return nil
}

// notify surfaces a message in the tmux status line (display-message), matching
// the shell's user feedback for keybinding-driven actions.
func (m *Manager) notify(msg string) { _ = m.tmux.Run("display-message", msg) }

func (m *Manager) savePref(workdir, agent string) {
	if workdir == "" {
		return
	}
	_ = m.prefs.Set("agent."+workdir, agent)
}

func (m *Manager) getOpt(name string) string {
	v, err := m.tmux.Output("show-option", "-t", m.session, "-v", name)
	if err != nil {
		return ""
	}
	return v
}

func (m *Manager) setOpt(name, value string) {
	_ = m.tmux.Run("set-option", "-t", m.session, name, value)
}

// paneExists reports whether pane (a tmux pane id) is still present in the dev
// window.
func (m *Manager) paneExists(pane string) bool {
	if pane == "" {
		return false
	}
	out, err := m.tmux.Output("list-panes", "-t", m.session+":dev", "-F", "#{pane_id}")
	if err != nil {
		return false
	}
	for _, id := range strings.Fields(out) {
		if id == pane {
			return true
		}
	}
	return false
}

// cycleIndex returns the next index when stepping through a list of count items
// from currentIdx. currentIdx of -1 (current agent not in the list) starts the
// cycle from the ends.
func cycleIndex(count, currentIdx int, forward bool) int {
	if count == 0 {
		return -1
	}
	idx := currentIdx
	if forward {
		if idx++; idx >= count {
			idx = 0
		}
	} else {
		if idx--; idx < 0 {
			idx = count - 1
		}
	}
	return idx
}

// indexOf returns the position of want in list, or -1 if absent.
func indexOf(list []string, want string) int {
	for i, v := range list {
		if v == want {
			return i
		}
	}
	return -1
}
