package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/drjzlyan/karya/internal/assets"
	"github.com/drjzlyan/karya/internal/config"
)

// cmdShell is the interactive shell karya launches inside its tmux panes (wired
// as tmux's default-command). It runs the user's OWN shell and rc unchanged, then
// layers karya's starship prompt on top using karya-owned init files — so panes
// get a consistent prompt and karya's managed tools on PATH without karya ever
// writing to the user's ~/.zshrc or ~/.bashrc. When starship isn't installed or
// the shell isn't zsh/bash, it execs the plain shell, so a pane always works.
//
// It replaces the current process with the shell via exec, so the pane's shell is
// a direct child of tmux (no lingering karya wrapper process).
func cmdShell(_ []string) int {
	p := config.Resolve()
	// Put karya's managed tools (starship, fzf, lazygit, …) on PATH for the pane,
	// mirroring what every other karya-spawned process gets.
	p.ActivateManagedEnv()
	// Best-effort: make sure the shell-init files exist even if the pane was somehow
	// spawned before `karya install` extracted them. A failure just means no prompt
	// wiring — the plain-shell fallback below still runs.
	hasInit := assets.ExtractShellInit(p.ShellInitDir()) == nil

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = defaultShell()
	}

	argv, env := buildShellInvocation(p, shell, hasInit && starshipAvailable())
	if err := syscall.Exec(argv[0], argv, env); err != nil {
		// Last resort: a bare shell, so the pane is never left dead.
		_ = syscall.Exec(shell, []string{shell}, os.Environ())
		return 1
	}
	return 0
}

// buildShellInvocation returns the argv and environment for the pane shell. When
// wire is true and the shell is zsh/bash, it points the shell at karya's init
// files (ZDOTDIR for zsh, --rcfile for bash) and pins STARSHIP_CONFIG to karya's
// prompt config. Otherwise it returns the plain interactive shell unchanged. It is
// a pure function of its inputs so the wiring is unit-testable without a shell.
func buildShellInvocation(p config.Paths, shell string, wire bool) (argv []string, env []string) {
	env = os.Environ()
	if wire {
		switch filepath.Base(shell) {
		case "zsh":
			// ZDOTDIR makes zsh read karya's .zshrc (which sources the user's rc,
			// then inits starship). -i forces an interactive shell.
			env = append(env,
				"STARSHIP_CONFIG="+p.StarshipConfig(),
				"ZDOTDIR="+p.ShellInitDir(),
			)
			return []string{shell, "-i"}, env
		case "bash":
			// --rcfile points bash at karya's rcfile for interactive shells; the
			// rcfile sources the user's ~/.bashrc then inits starship.
			env = append(env, "STARSHIP_CONFIG="+p.StarshipConfig())
			return []string{shell, "--rcfile", filepath.Join(p.ShellInitDir(), "bashrc"), "-i"}, env
		}
	}
	return []string{shell}, env
}

// starshipAvailable reports whether the starship prompt resolves on the (managed)
// PATH, so karya only wires the prompt when it can actually render.
func starshipAvailable() bool {
	_, err := exec.LookPath("starship")
	return err == nil
}

// defaultShell is the fallback interactive shell when $SHELL is unset (rare inside
// a tmux pane, which sets it). zsh on macOS, bash elsewhere.
func defaultShell() string {
	if runtime.GOOS == "darwin" {
		return "/bin/zsh"
	}
	return "/bin/bash"
}
