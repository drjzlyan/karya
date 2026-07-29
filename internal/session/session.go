// Package session builds and manages karya's tmux IDE layout — a faithful port
// of dotfiles/scripts/dev.sh, adapted to karya's isolated tmux server and the
// NVIM_APPNAME/EDITOR routing that keeps everything namespaced to karya.
//
// Layout (window "dev"):
//
//	┌──────────────────────┬──────────────────┐
//	│                      │  agent           │
//	│      editor (nvim)   ├──────────────────┤
//	│                      │  build / test    │
//	└──────────────────────┴──────────────────┘
//
// A second window "git" runs lazygit. Pane IDs and agent state live in tmux
// session options (@ide_*), matching the original scheme so behavior carries over.
package session

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/drjzlyan/karya/internal/agent"
	"github.com/drjzlyan/karya/internal/tmuxx"
)

// Options configures a dev session.
type Options struct {
	Name     string   // tmux session name
	Workdir  string   // working directory for all panes
	Agent    string   // resolved agent name, or agent.None
	Detected []string // detected agents (stored for in-session switching)
	Kill     bool     // kill an existing session and recreate it
}

// Dev launches (or attaches to) the IDE session. On success it replaces the
// process via tmux attach and does not return.
func Dev(t *tmuxx.Tmux, o Options) error {
	if !tmuxx.Available() {
		return fmt.Errorf("tmux is not installed; run `karya doctor`")
	}

	if t.HasSession(o.Name) {
		if o.Kill {
			_ = t.Run("kill-session", "-t", o.Name)
		} else {
			fmt.Printf("Session %q already exists. Attaching.\n", o.Name)
			return t.Attach(o.Name + ":dev")
		}
	}

	dev := o.Name + ":dev"
	p1, p2, p3 := dev+".1", dev+".2", dev+".3"

	// ── Window 1: dev ──────────────────────────────────────────────
	if err := t.Run("new-session", "-d", "-s", o.Name, "-n", "dev", "-c", o.Workdir); err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	// Session environment: editor actions route into the editor pane and Neovim
	// stays namespaced under NVIM_APPNAME=karya. New panes inherit these.
	for _, kv := range t.Env {
		if name, val, ok := strings.Cut(kv, "="); ok {
			_ = t.Run("set-environment", "-t", o.Name, name, val)
		}
	}

	// Editor pane (left, 65%). Set NVIM_APPNAME explicitly since this pane was
	// spawned before the session environment was applied.
	_ = t.Run("select-pane", "-t", p1, "-T", "editor")
	_ = t.Run("send-keys", "-t", p1, "NVIM_APPNAME=karya nvim", "Enter")

	// Right column: agent (top) over build/test (bottom).
	_ = t.Run("split-window", "-h", "-l", "35%", "-t", p1, "-c", o.Workdir)
	_ = t.Run("split-window", "-v", "-l", "40%", "-t", p2, "-c", o.Workdir)

	// Agent pane.
	_ = t.Run("select-pane", "-t", p2, "-T", "agent")
	if o.Agent != "" && o.Agent != agent.None && agent.Available(o.Agent) {
		time.Sleep(time.Second) // let the shell settle before typing
		_ = t.Run("send-keys", "-t", p2, o.Agent, "Enter")
	} else if o.Agent != "" && o.Agent != agent.None {
		fmt.Fprintf(os.Stderr, "warning: agent %q not found on PATH; opening a shell\n", o.Agent)
	}

	// Build/test pane.
	_ = t.Run("select-pane", "-t", p3, "-T", "build/test")

	// Focus the editor.
	_ = t.Run("select-pane", "-t", p1)

	// Persist state for in-session agent management (Phase 2 reads these).
	agents := strings.Join(o.Detected, " ")
	if agents == "" {
		agents = o.Agent
	}
	setOpt(t, o.Name, "@ide_agents", agents)
	setOpt(t, o.Name, "@ide_current_agent", o.Agent)
	setOpt(t, o.Name, "@ide_workdir", o.Workdir)
	setOpt(t, o.Name, "@ide_agent_pane", paneID(t, p2))
	setOpt(t, o.Name, "@ide_editor_pane", paneID(t, p1))
	setOpt(t, o.Name, "@ide_shell_pane", paneID(t, p3))

	// ── Window 2: git ──────────────────────────────────────────────
	if agent.Available("lazygit") {
		_ = t.Run("new-window", "-t", o.Name, "-n", "git", "-c", o.Workdir, "lazygit")
	}

	return t.Attach(dev)
}

// Quit cleanly tears down a session: ask Neovim to quit, then kill the session.
func Quit(t *tmuxx.Tmux, name string) error {
	if !t.HasSession(name) {
		fmt.Printf("Session %q does not exist.\n", name)
		return nil
	}
	_ = t.Run("send-keys", "-t", name+":dev.1", ":qa!", "Enter")
	time.Sleep(time.Second)
	if err := t.Run("kill-session", "-t", name); err != nil {
		return err
	}
	fmt.Printf("Session %q terminated.\n", name)
	return nil
}

func setOpt(t *tmuxx.Tmux, session, name, value string) {
	_ = t.Run("set-option", "-t", session, name, value)
}

func paneID(t *tmuxx.Tmux, target string) string {
	id, err := t.Output("display-message", "-p", "-t", target, "#{pane_id}")
	if err != nil {
		return ""
	}
	return id
}
