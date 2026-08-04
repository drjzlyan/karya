package cli

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/drjzlyan/karya/internal/config"
)

// envHas reports whether env contains an exact "NAME=VALUE" entry.
func envHas(env []string, kv string) bool { return slices.Contains(env, kv) }

func TestBuildShellInvocationWiresStarship(t *testing.T) {
	p := config.Paths{Config: "/k/config"}
	zdotdir := "ZDOTDIR=" + p.ShellInitDir()
	starship := "STARSHIP_CONFIG=" + p.StarshipConfig()

	t.Run("zsh wired", func(t *testing.T) {
		argv, env := buildShellInvocation(p, "/bin/zsh", true)
		if want := []string{"/bin/zsh", "-i"}; !slices.Equal(argv, want) {
			t.Errorf("argv = %v, want %v", argv, want)
		}
		if !envHas(env, zdotdir) {
			t.Errorf("env missing %q", zdotdir)
		}
		if !envHas(env, starship) {
			t.Errorf("env missing %q", starship)
		}
	})

	t.Run("bash wired", func(t *testing.T) {
		argv, env := buildShellInvocation(p, "/usr/bin/bash", true)
		wantRC := filepath.Join(p.ShellInitDir(), "bashrc")
		if want := []string{"/usr/bin/bash", "--rcfile", wantRC, "-i"}; !slices.Equal(argv, want) {
			t.Errorf("argv = %v, want %v", argv, want)
		}
		if !envHas(env, starship) {
			t.Errorf("env missing %q", starship)
		}
	})
}

func TestBuildShellInvocationFallsBackToPlainShell(t *testing.T) {
	p := config.Paths{Config: "/k/config"}

	// starship unavailable (wire=false): plain shell, no karya env injected.
	t.Run("no starship", func(t *testing.T) {
		argv, env := buildShellInvocation(p, "/bin/zsh", false)
		if want := []string{"/bin/zsh"}; !slices.Equal(argv, want) {
			t.Errorf("argv = %v, want plain shell %v", argv, want)
		}
		for _, e := range env {
			if strings.HasPrefix(e, "ZDOTDIR=/k/config") || strings.HasPrefix(e, "STARSHIP_CONFIG=") {
				t.Errorf("plain shell must not inject karya env, got %q", e)
			}
		}
	})

	// Unsupported shell (fish) with starship present still falls back to plain,
	// so the pane never breaks on shells karya can't wire.
	t.Run("unsupported shell", func(t *testing.T) {
		argv, _ := buildShellInvocation(p, "/opt/homebrew/bin/fish", true)
		if want := []string{"/opt/homebrew/bin/fish"}; !slices.Equal(argv, want) {
			t.Errorf("argv = %v, want plain fish %v", argv, want)
		}
	})
}
