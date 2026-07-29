//go:build integration

// Integration tests drive the real tmux binary on a throwaway socket to prove
// the Manager's layout operations (respawn/cycle/reset) work end to end. They
// live in the external test package so they may import internal/session (which
// imports internal/agent) without a build cycle. Gated behind the `integration`
// tag; CI installs tmux and runs `go test -tags=integration ./...`.
package agent_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/drjzlyan/karya/internal/agent"
	"github.com/drjzlyan/karya/internal/assets"
	"github.com/drjzlyan/karya/internal/prefs"
	"github.com/drjzlyan/karya/internal/session"
	"github.com/drjzlyan/karya/internal/tmuxx"
)

func newTestTmux(t *testing.T) *tmuxx.Tmux {
	t.Helper()
	if !tmuxx.Available() {
		t.Skip("tmux not installed")
	}
	conf := filepath.Join(t.TempDir(), "tmux.conf")
	if err := assets.ExtractTmuxConf(conf, "/bin/true"); err != nil {
		t.Fatalf("extract tmux.conf: %v", err)
	}
	socket := fmt.Sprintf("karya-itest-%d", time.Now().UnixNano())
	tx := tmuxx.New(socket, conf, []string{"NVIM_APPNAME=karya", "EDITOR=/bin/true edit"})
	t.Cleanup(func() { _ = tx.Run("kill-server") })
	return tx
}

func devPaneTitles(t *testing.T, tx *tmuxx.Tmux, name string) string {
	t.Helper()
	out, err := tx.Output("list-panes", "-t", name+":dev", "-F", "#{pane_index} #{pane_title}")
	if err != nil {
		t.Fatalf("list-panes: %v", err)
	}
	return out
}

// TestNextCyclesAgentState builds a session, seeds two agents, and asserts Next
// advances @ide_current_agent, saves the preference, and keeps the 3-pane layout
// intact (the agent pane is respawned in place, not rebuilt).
func TestNextCyclesAgentState(t *testing.T) {
	tx := newTestTmux(t)
	name := "karyanext"
	workdir := t.TempDir()
	if err := session.Build(tx, session.Options{Name: name, Workdir: workdir, Agent: "none"}); err != nil {
		t.Fatalf("Build: %v", err)
	}
	// Neither agent is on PATH, so launch respawns the pane but sends no command
	// — exactly the state transition we want to assert without real agent CLIs.
	if err := tx.Run("set-option", "-t", name, "@ide_agents", "none faux"); err != nil {
		t.Fatalf("seed agents: %v", err)
	}

	store := prefs.New(filepath.Join(t.TempDir(), "prefs"))
	m := agent.NewManager(tx, store, name, "/bin/true")
	if err := m.Next(); err != nil {
		t.Fatalf("Next: %v", err)
	}

	if got, _ := tx.Output("show-options", "-t", name, "-v", "@ide_current_agent"); got != "faux" {
		t.Errorf("@ide_current_agent = %q, want faux", got)
	}
	if got := store.Get("agent." + workdir); got != "faux" {
		t.Errorf("saved pref = %q, want faux", got)
	}
	if titles := devPaneTitles(t, tx, name); strings.Count(titles, "\n") != 2 { // 3 panes
		t.Errorf("layout not preserved after Next; panes:\n%s", titles)
	}
}

// TestResetRebuildsLayout removes a pane, then asserts Reset restores the full
// editor / agent / build-test layout.
func TestResetRebuildsLayout(t *testing.T) {
	tx := newTestTmux(t)
	name := "karyareset"
	if err := session.Build(tx, session.Options{Name: name, Workdir: t.TempDir(), Agent: "none"}); err != nil {
		t.Fatalf("Build: %v", err)
	}
	// Tear the layout: drop the build/test pane so only two remain.
	if err := tx.Run("kill-pane", "-t", name+":dev.3"); err != nil {
		t.Fatalf("kill-pane: %v", err)
	}

	m := agent.NewManager(tx, prefs.New(filepath.Join(t.TempDir(), "prefs")), name, "/bin/true")
	if err := m.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	titles := devPaneTitles(t, tx, name)
	for _, want := range []string{"1 editor", "2 agent", "3 build/test"} {
		if !strings.Contains(titles, want) {
			t.Errorf("after Reset, panes missing %q; got:\n%s", want, titles)
		}
	}
}
