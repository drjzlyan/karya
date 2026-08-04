// Package config resolves karya's isolated, XDG-based directories.
//
// Isolation is the defining constraint of karya (see PLAN.md §2): every
// path karya touches lives under a karya-owned prefix, and Neovim/tmux are
// launched with karya-specific env so the user's own configuration is never
// read or written. All other packages MUST obtain paths from here rather than
// hardcoding "~/.config/..." so the isolation guarantee holds in one place.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// ActivateManagedEnv configures the current karya process to use the tools it
// installed into its isolated prefix. It prepends the managed tool bin and mise
// shims to PATH (guarded against duplicates) and exports the MISE_* variables
// so those shims resolve karya's isolated runtimes rather than the user's global
// mise. This brings karya's own tool lookups — doctor version probes, `karya
// edit`'s Neovim, tmux/agent detection — in line with the child processes it
// launches via Paths.Env. It mutates only the karya process environment; the
// user's shell PATH, Homebrew, and global mise are never touched.
func (p Paths) ActivateManagedEnv() {
	cur := os.Getenv("PATH")
	have := make(map[string]bool)
	for _, d := range strings.Split(cur, string(os.PathListSeparator)) {
		have[d] = true
	}
	var add []string
	for _, d := range p.managedPathDirs() {
		if !have[d] {
			add = append(add, d)
		}
	}
	if len(add) > 0 {
		next := strings.Join(add, string(os.PathListSeparator))
		if cur != "" {
			next += string(os.PathListSeparator) + cur
		}
		_ = os.Setenv("PATH", next)
	}
	// Pin mise to karya's prefix so shim-backed tools resolve karya's runtimes.
	for _, kv := range p.MiseEnv() {
		if name, val, ok := strings.Cut(kv, "="); ok {
			_ = os.Setenv(name, val)
		}
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

// ShellInitDir holds karya's own shell startup files (a zsh .zshrc and a bash
// rcfile). The pane shell (`karya shell`) points ZDOTDIR / --rcfile here so it can
// source the user's real rc and then layer karya's starship prompt on top —
// without karya ever writing to the user's own ~/.zshrc or ~/.bashrc.
func (p Paths) ShellInitDir() string { return filepath.Join(p.Config, "shell") }

// StarshipConfig is karya's own starship config, pinned via STARSHIP_CONFIG in the
// pane shell so the prompt is deterministic and independent of any user starship
// config.
func (p Paths) StarshipConfig() string { return filepath.Join(p.ShellInitDir(), "starship.toml") }

// PrefsFile stores per-project preferences (e.g. chosen agent), key=value.
func (p Paths) PrefsFile() string { return filepath.Join(p.Data, "prefs") }

// ToolsStateFile records per-tool metadata such as the last time karya checked a
// tool for updates, key=value under the state dir.
func (p Paths) ToolsStateFile() string { return filepath.Join(p.State, "tools.state") }

// ToolsBin is karya's private tool prefix; managed LSPs/formatters live here so
// karya never mutates Homebrew or the user's global environment.
func (p Paths) ToolsBin() string { return filepath.Join(p.Data, "tools", "bin") }

// ToolsDir is the root of karya's private tool prefix (ToolsBin is ToolsDir/bin).
// Standalone tool payloads (e.g. jdtls, lombok.jar) live directly under it.
func (p Paths) ToolsDir() string { return filepath.Join(p.Data, "tools") }

// toolCategories are the per-category tool prefixes under ToolsDir. Grouping
// installs by role (core CLI, docs, per language) keeps binaries from scattering
// across one flat bin dir and makes per-category update/repair tractable. The
// list is fixed so PATH construction is deterministic; unused dirs on PATH are
// harmlessly ignored by the OS until a tool populates them.
var toolCategories = []string{"core", "docs", "python", "typescript", "go", "rust", "java", "cpp"}

// ToolCategoryDir returns the install root for a named tool category (e.g.
// "core", "docs", "go"). Tools whose install location has no category fall back
// to the shared ToolsDir/ToolsBin.
func (p Paths) ToolCategoryDir(name string) string {
	return filepath.Join(p.Data, "tools", name)
}

// ToolCategoryBin returns the bin dir for a named tool category.
func (p Paths) ToolCategoryBin(name string) string {
	return filepath.Join(p.ToolCategoryDir(name), "bin")
}

// ToolBinDirs returns every managed tool bin dir in PATH priority order: the
// legacy shared bin (kept forever so existing installs resolve) followed by each
// category bin. mise shims are appended separately by the env builders.
func (p Paths) ToolBinDirs() []string {
	dirs := make([]string, 0, len(toolCategories)+1)
	dirs = append(dirs, p.ToolsBin())
	for _, c := range toolCategories {
		dirs = append(dirs, p.ToolCategoryBin(c))
	}
	return dirs
}

// DownloadsDir is karya-owned staging for downloaded archives (jdtls, VSIX),
// keeping transient files inside the karya prefix instead of the system temp.
func (p Paths) DownloadsDir() string { return filepath.Join(p.Data, "downloads") }

// ToolsLogsDir holds install/update logs for later diagnosis.
func (p Paths) ToolsLogsDir() string { return filepath.Join(p.Data, "logs") }

// managedPathDirs is the ordered set of dirs karya prepends to PATH so its
// isolated toolchain resolves: every tool bin dir, then mise shims.
func (p Paths) managedPathDirs() []string {
	return append(p.ToolBinDirs(), p.MiseShims())
}

// LanguagesFile records the selected languages and runtime versions in
// key=value form (e.g. python=3.14,3.13). It is the source of truth from which
// karya regenerates its isolated mise config.
func (p Paths) LanguagesFile() string { return filepath.Join(p.Data, "languages.local") }

// MiseConfig is karya's generated, isolated mise config file. It is pinned via
// MISE_GLOBAL_CONFIG_FILE so karya's runtimes never touch the user's own
// ~/.config/mise/config.toml.
func (p Paths) MiseConfig() string { return filepath.Join(p.Config, "mise", "config.toml") }

// NvimData is Neovim's data dir under karya's prefix. With NVIM_APPNAME=karya/nvim
// and Data = XDG_DATA_HOME/karya, Neovim's data lands at Data/nvim. Unlike
// NvimConfig (which karya overwrites wholesale on config extraction), this dir
// persists, so karya writes the tool manifest here for the editor to read.
func (p Paths) NvimData() string { return filepath.Join(p.Data, "nvim") }

// ToolsManifest is the JSON map of resolved tool executables karya writes for the
// embedded Neovim config, so the editor resolves managed tools by absolute path
// instead of guessing at global locations. It lives in Neovim's data dir so
// stdpath("data").."/karya-tools.json" finds it.
func (p Paths) ToolsManifest() string { return filepath.Join(p.NvimData(), "karya-tools.json") }

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

// ShellEnv returns POSIX-sh shell integration a user opts into with
// `eval "$(karya shellenv)"` (never written to an rc file by karya). It puts the
// karya bin dir on PATH so `karya` resolves, routes $EDITOR through karya, and
// adds a short `k` alias. It deliberately does NOT prepend the managed tool bin or
// mise shims: that toolchain is session-scoped (see Env) so it never shadows the
// user's global tools. The PATH edit is guarded so repeated evals don't stack
// duplicates. karyaBin is the absolute path to the running binary.
func (p Paths) ShellEnv(karyaBin string) string {
	edit := karyaBin + " edit"
	return fmt.Sprintf(`# karya shell integration — opt in by adding to your shell rc:
#   eval "$(karya shellenv)"
case ":$PATH:" in
  *":%[1]s:"*) ;;
  *) export PATH="%[1]s:$PATH" ;;
esac
export EDITOR="%[2]s"
export VISUAL="%[2]s"
export GIT_EDITOR="%[2]s"
alias k="%[3]s"
`, p.Bin, edit, karyaBin)
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
	path := strings.Join(p.managedPathDirs(), string(os.PathListSeparator))
	if cur := os.Getenv("PATH"); cur != "" {
		path += string(os.PathListSeparator) + cur
	}
	env = append(env, "PATH="+path)
	return env
}

// EnvForProject is Env plus per-project isolation: it trusts the project's own
// mise/.tool-versions config (via MISE_TRUSTED_CONFIG_PATHS) so that, when karya
// runs tools with the project as the working directory, mise layers the project's
// runtime versions over karya's global managed ones — without touching the user's
// global mise trust store. projectRoot is the detected project directory; an empty
// projectRoot yields the same result as Env.
func (p Paths) EnvForProject(karyaBin, projectRoot string) []string {
	env := p.Env(karyaBin)
	if projectRoot != "" {
		env = append(env, "MISE_TRUSTED_CONFIG_PATHS="+projectRoot)
	}
	return env
}
