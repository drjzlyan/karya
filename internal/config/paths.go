// Package config resolves karya's isolated, XDG-based directories.
//
// Isolation is the defining constraint of karya (see PLAN.md §2): every
// path karya touches lives under a karya-owned prefix, and Neovim/tmux are
// launched with karya-specific env so the user's own configuration is never
// read or written. All other packages MUST obtain paths from here rather than
// hardcoding "~/.config/..." so the isolation guarantee holds in one place.
package config

import (
	"os"
	"path/filepath"
)

// AppName is the karya prefix used for all XDG subdirectories.
const AppName = "karya"

// NvimAppName is the value of NVIM_APPNAME. The trailing "/nvim" nests Neovim's
// config/data/state/cache one level below the karya prefix (e.g.
// ~/.config/karya/nvim), keeping the editor's files separate from karya's own
// tmux.conf/prefs/tools while staying fully inside the isolated karya tree. It
// deliberately matches NvimConfig() so Neovim reads exactly what karya extracts.
const NvimAppName = AppName + "/nvim"

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
// NVIM_APPNAME=karya/nvim, Neovim reads its config from ~/.config/karya/nvim, so
// the extracted tree lives there, mirroring a standard ~/.config/nvim layout.
func (p Paths) NvimConfig() string { return filepath.Join(p.Config, "nvim") }

// TmuxConf is the extracted karya tmux configuration used with `tmux -f`.
func (p Paths) TmuxConf() string { return filepath.Join(p.Config, "tmux.conf") }

// PrefsFile stores per-project preferences (e.g. chosen agent), key=value.
func (p Paths) PrefsFile() string { return filepath.Join(p.Data, "prefs") }

// ToolsBin is karya's private tool prefix; managed LSPs/formatters live here so
// karya never mutates Homebrew or the user's global environment.
func (p Paths) ToolsBin() string { return filepath.Join(p.Data, "tools", "bin") }

// ToolsDir is the root of karya's private tool prefix (ToolsBin is ToolsDir/bin).
// Standalone tool payloads (e.g. jdtls, lombok.jar) live directly under it.
func (p Paths) ToolsDir() string { return filepath.Join(p.Data, "tools") }

// LanguagesFile records the selected languages and runtime versions in
// key=value form (e.g. python=3.14,3.13). It is the source of truth from which
// karya regenerates its isolated mise config.
func (p Paths) LanguagesFile() string { return filepath.Join(p.Data, "languages.local") }

// MiseConfig is karya's generated, isolated mise config file. It is pinned via
// MISE_GLOBAL_CONFIG_FILE so karya's runtimes never touch the user's own
// ~/.config/mise/config.toml.
func (p Paths) MiseConfig() string { return filepath.Join(p.Config, "mise", "config.toml") }

// MiseData is karya's isolated mise data dir (installed runtimes + shims).
func (p Paths) MiseData() string { return filepath.Join(p.Data, "mise") }

// MiseCache is karya's isolated mise cache dir.
func (p Paths) MiseCache() string { return filepath.Join(p.Cache, "mise") }

// MiseShims is where mise writes runtime shims; kept on PATH inside sessions.
func (p Paths) MiseShims() string { return filepath.Join(p.MiseData(), "shims") }

// MiseEnv pins mise entirely inside the karya prefix so version selection and
// `mise install` never read or write the user's global mise config, data, or
// cache. Callers append these to os.Environ() when invoking mise.
func (p Paths) MiseEnv() []string {
	return []string{
		"MISE_GLOBAL_CONFIG_FILE=" + p.MiseConfig(),
		"MISE_DATA_DIR=" + p.MiseData(),
		"MISE_CACHE_DIR=" + p.MiseCache(),
		"MISE_STATE_DIR=" + filepath.Join(p.State, "mise"),
	}
}

// TmuxSocket is the dedicated tmux server label (`tmux -L`) that isolates
// karya's sessions from the user's default tmux server.
const TmuxSocket = "karya"

// Env returns the environment overrides karya applies to child processes
// (Neovim, tmux, agents, git) so editor actions route into the IDE and Neovim
// stays namespaced. karyaBin is the absolute path to the running karya binary so
// $EDITOR resolves regardless of PATH. Callers append these to os.Environ().
func (p Paths) Env(karyaBin string) []string {
	edit := karyaBin + " edit"
	env := []string{
		"NVIM_APPNAME=" + NvimAppName,
		"EDITOR=" + edit,
		"VISUAL=" + edit,
		"GIT_EDITOR=" + edit,
	}
	// Pin mise, and put karya's managed tool bin + mise shims ahead of the user's
	// PATH so the isolated toolchain wins inside karya sessions — without ever
	// mutating the user's own PATH, Homebrew, or global mise (see PLAN.md §2, §6.4).
	env = append(env, p.MiseEnv()...)
	path := p.ToolsBin() + string(os.PathListSeparator) + p.MiseShims()
	if cur := os.Getenv("PATH"); cur != "" {
		path += string(os.PathListSeparator) + cur
	}
	env = append(env, "PATH="+path)
	return env
}
