//go:build integration

// Integration tests exercise the real tmux binary on a throwaway server socket.
// They are excluded from the default `go test` run and gated behind the
// `integration` build tag so unit tests stay hermetic. CI installs tmux and runs
// these with `go test -tags=integration ./...`.
package session

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/drjzlyan/karya/internal/assets"
	"github.com/drjzlyan/karya/internal/tmuxx"
)

// karyaStub writes a stand-in for the karya binary that tmux's default-command
// (`karya shell`) can exec: given "shell" it execs a real shell so panes stay
// alive, as production does; any other subcommand is a no-op success (matching the
// old /bin/true placeholder used for the run-shell key bindings).
func karyaStub(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "karya")
	script := "#!/bin/sh\n[ \"$1\" = shell ] && exec \"${SHELL:-/bin/sh}\"\nexit 0\n"
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// newTestTmux returns a Tmux bound to a unique, disposable server socket AND
// karya's own extracted tmux config (`-f`). Using karya's config is what makes
// the test both faithful to production (1-based window/pane indexes, which
// session.Build relies on) and fully isolated: without `-f`, tmux would fall
// back to reading the user's ~/.tmux.conf, which would make the test both
// non-hermetic and index-dependent on the developer's machine.
func newTestTmux(t *testing.T) *tmuxx.Tmux {
	t.Helper()
	if !tmuxx.Available() {
		t.Skip("tmux not installed")
	}
	conf := filepath.Join(t.TempDir(), "tmux.conf")
	if err := assets.ExtractTmuxConf(conf, karyaStub(t)); err != nil {
		t.Fatalf("extract tmux.conf: %v", err)
	}
	socket := fmt.Sprintf("karya-itest-%d", time.Now().UnixNano())
	tx := tmuxx.New(socket, conf, []string{"NVIM_APPNAME=karya/nvim", "EDITOR=/bin/true edit"})
	t.Cleanup(func() { _ = tx.Run("kill-server") })
	return tx
}

// TestBuildLayout asserts Build creates the expected windows, panes, titles,
// isolated environment, and @ide_* state — the Phase 1 contract.
func TestBuildLayout(t *testing.T) {
	tx := newTestTmux(t)
	name := "karyatest"
	workdir := t.TempDir()

	if err := Build(tx, Options{Name: name, Workdir: workdir, Agent: "none"}); err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Three panes in the dev window with the expected titles.
	panes, err := tx.Output("list-panes", "-t", name+":dev", "-F", "#{pane_index} #{pane_title}")
	if err != nil {
		t.Fatalf("list-panes: %v", err)
	}
	for _, want := range []string{"1 editor", "2 agent", "3 build/test"} {
		if !strings.Contains(panes, want) {
			t.Errorf("panes missing %q; got:\n%s", want, panes)
		}
	}

	// Isolated session environment.
	env, err := tx.Output("show-environment", "-t", name)
	if err != nil {
		t.Fatalf("show-environment: %v", err)
	}
	if !strings.Contains(env, "NVIM_APPNAME=karya/nvim") {
		t.Errorf("session env missing NVIM_APPNAME=karya/nvim; got:\n%s", env)
	}

	// @ide_* state options are populated.
	for _, opt := range []string{"@ide_current_agent", "@ide_editor_pane", "@ide_workdir"} {
		if v, err := tx.Output("show-options", "-t", name, "-v", opt); err != nil || v == "" {
			t.Errorf("option %s not set (v=%q, err=%v)", opt, v, err)
		}
	}
}

// TestBuildIsolation asserts the session lives only on the disposable socket and
// never leaks onto the user's default tmux server.
func TestBuildIsolation(t *testing.T) {
	tx := newTestTmux(t)
	name := "karyaiso"
	if err := Build(tx, Options{Name: name, Workdir: t.TempDir(), Agent: "none"}); err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !tx.HasSession(name) {
		t.Fatalf("session %q not found on the test socket", name)
	}
	// The user's default tmux server (raw `tmux`, no -L) must not see it. Skip
	// when running inside a tmux session to avoid touching the caller's server.
	if os.Getenv("TMUX") == "" {
		if exec.Command("tmux", "has-session", "-t", name).Run() == nil {
			t.Errorf("session %q leaked onto the default tmux server", name)
		}
	}
}
