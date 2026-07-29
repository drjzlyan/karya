package editor

import (
	"strings"
	"testing"
)

func TestSyncPluginsArgs(t *testing.T) {
	args := syncPluginsArgs()
	got := strings.Join(args, " ")
	// Headless plugin install/update, then quit-all. Lazy! is the non-interactive
	// form so the process exits without waiting on the UI.
	want := "--headless +Lazy! sync +qa"
	if got != want {
		t.Errorf("syncPluginsArgs() = %q, want %q", got, want)
	}
}

func TestSyncPluginsNoNvimIsNoOp(t *testing.T) {
	// Point PATH at an empty dir so nvim cannot be found; SyncPlugins must treat a
	// missing editor as a benign no-op (lazy.nvim self-bootstraps on first launch).
	t.Setenv("PATH", t.TempDir())
	if err := SyncPlugins(nil); err != nil {
		t.Errorf("SyncPlugins with no nvim on PATH = %v, want nil", err)
	}
}
