// Package config resolves karya's isolated, XDG-based directories.
//
// Isolation is the defining constraint of karya (see docs/PLAN.md §2): every
// path karya touches lives under a karya-owned prefix, and Neovim/tmux are
// launched with karya-specific env so the user's own configuration is never
// read or written. All other packages MUST obtain paths from here rather than
// hardcoding "~/.config/..." so the isolation guarantee holds in one place.
package config

import (
	"os"
	"path/filepath"
)

// AppName is used for the XDG subdirectories and, crucially, as NVIM_APPNAME so
// Neovim isolates its config/data/state/cache under a karya-specific namespace.
const AppName = "karya"

// Paths is the set of karya-owned directories, all namespaced by AppName.
type Paths struct {
	// Config: ~/.config/karya (extracted nvim config, tmux.conf, …)
	Config string
	// Data: ~/.local/share/karya (tools, languages.local, prefs, mise, …)
	Data string
	// State: ~/.local/state/karya
	State string
	// Cache: ~/.cache/karya
	Cache string
	// Bin: ~/.local/bin (where the single karya binary is installed)
	Bin string
}

// xdg returns the value of an XDG_* env var or a HOME-relative fallback.
func xdg(env, fallback string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, fallback)
}

// Resolve computes the karya paths from the environment. It does not create the
// directories; call EnsureDirs for that.
func Resolve() Paths {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return Paths{
		Config: filepath.Join(xdg("XDG_CONFIG_HOME", ".config"), AppName),
		Data:   filepath.Join(xdg("XDG_DATA_HOME", filepath.Join(".local", "share")), AppName),
		State:  filepath.Join(xdg("XDG_STATE_HOME", filepath.Join(".local", "state")), AppName),
		Cache:  filepath.Join(xdg("XDG_CACHE_HOME", ".cache"), AppName),
		Bin:    filepath.Join(home, ".local", "bin"),
	}
}

// EnsureDirs creates all karya-owned directories if they do not exist.
func (p Paths) EnsureDirs() error {
	for _, d := range []string{p.Config, p.Data, p.State, p.Cache, p.Bin} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// NvimConfig is where the embedded Neovim config is extracted. With
// NVIM_APPNAME=karya, Neovim reads from ~/.config/karya, so the config lives at
// ~/.config/karya/nvim mirroring a standard ~/.config/nvim layout.
func (p Paths) NvimConfig() string { return filepath.Join(p.Config, "nvim") }

// TmuxConf is the extracted karya tmux configuration used with `tmux -f`.
func (p Paths) TmuxConf() string { return filepath.Join(p.Config, "tmux.conf") }

// PrefsFile stores per-project preferences (e.g. chosen agent), key=value.
func (p Paths) PrefsFile() string { return filepath.Join(p.Data, "prefs") }

// ToolsBin is karya's private tool prefix; managed LSPs/formatters live here so
// karya never mutates Homebrew or the user's global environment.
func (p Paths) ToolsBin() string { return filepath.Join(p.Data, "tools", "bin") }

// TmuxSocket is the dedicated tmux server label (`tmux -L`) that isolates
// karya's sessions from the user's default tmux server.
const TmuxSocket = "karya"

// Env returns the environment overrides karya applies to child processes
// (Neovim, tmux, agents, git) so editor actions route into the IDE and Neovim
// stays namespaced. Callers append these to os.Environ().
func (p Paths) Env() []string {
	return []string{
		"NVIM_APPNAME=" + AppName,
		"EDITOR=karya edit",
		"VISUAL=karya edit",
		"GIT_EDITOR=karya edit",
	}
}
