package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestResolveIsolation asserts every karya path lives under the user's home in a
// karya-namespaced directory — the core isolation guarantee. If this breaks,
// karya may be writing where the user's own config lives.
func TestResolveIsolation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Clear XDG overrides so we exercise the HOME-relative fallbacks.
	for _, e := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"} {
		t.Setenv(e, "")
	}

	p := Resolve()
	for name, dir := range map[string]string{
		"Config": p.Config, "Data": p.Data, "State": p.State, "Cache": p.Cache,
	} {
		if !strings.HasPrefix(dir, home) {
			t.Errorf("%s = %q, want under HOME %q", name, dir, home)
		}
		if !strings.HasSuffix(dir, "/"+AppName) {
			t.Errorf("%s = %q, want namespaced by %q", name, dir, AppName)
		}
	}
	// Neovim config must sit inside the karya config dir so NVIM_APPNAME isolates it.
	if !strings.HasPrefix(p.NvimConfig(), p.Config) {
		t.Errorf("NvimConfig %q not under Config %q", p.NvimConfig(), p.Config)
	}
}

func TestEnvNamespacesNvimAndEditor(t *testing.T) {
	env := Paths{}.Env("/abs/karya")
	joined := strings.Join(env, "\n")
	for _, want := range []string{
		// karya/nvim (not bare karya) so Neovim reads the config extracted to
		// ~/.config/karya/nvim and isolates its data/state/cache under karya/nvim.
		"NVIM_APPNAME=" + NvimAppName,
		"EDITOR=/abs/karya edit",
		"VISUAL=/abs/karya edit",
		"GIT_EDITOR=/abs/karya edit",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("Env missing %q; got:\n%s", want, joined)
		}
	}
}

// TestMiseIsolation asserts karya's mise config, data, and cache all live under
// the karya prefix and that Env pins mise there and prepends karya's tool dirs to
// PATH — so `karya lang` never touches the user's global mise or Homebrew.
func TestMiseIsolation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, e := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"} {
		t.Setenv(e, "")
	}
	t.Setenv("PATH", "/usr/bin")

	p := Resolve()
	if !strings.HasPrefix(p.MiseConfig(), p.Config) {
		t.Errorf("MiseConfig %q not under Config %q", p.MiseConfig(), p.Config)
	}
	if !strings.HasPrefix(p.MiseData(), p.Data) {
		t.Errorf("MiseData %q not under Data %q", p.MiseData(), p.Data)
	}
	if !strings.HasPrefix(p.LanguagesFile(), p.Data) {
		t.Errorf("LanguagesFile %q not under Data %q", p.LanguagesFile(), p.Data)
	}

	env := strings.Join(p.Env("/abs/karya"), "\n")
	for _, want := range []string{
		"MISE_GLOBAL_CONFIG_FILE=" + p.MiseConfig(),
		"MISE_DATA_DIR=" + p.MiseData(),
		"PATH=" + p.ToolsBin(),
	} {
		if !strings.Contains(env, want) {
			t.Errorf("Env missing %q; got:\n%s", want, env)
		}
	}
	if !strings.Contains(env, p.MiseShims()) || !strings.HasSuffix(env, "/usr/bin") {
		t.Errorf("Env PATH must prepend tool dirs and preserve the user PATH; got:\n%s", env)
	}
}

// TestShellEnvIsOptInAndSafe asserts karya's shell integration only exposes where
// karya lives and sets the editor — it never leaks the session-scoped managed
// toolchain onto the user's global PATH (that stays inside karya sessions), and it
// mutates no user rc file (it is meant to be eval'd voluntarily).
func TestShellEnvIsOptInAndSafe(t *testing.T) {
	p := Paths{Bin: "/home/u/.local/bin", Data: "/home/u/.local/share/karya"}
	script := p.ShellEnv("/home/u/.local/bin/karya")

	if !strings.Contains(script, p.Bin) {
		t.Errorf("ShellEnv must put the karya bin dir on PATH; got:\n%s", script)
	}
	for _, want := range []string{
		"EDITOR=", "VISUAL=", "GIT_EDITOR=", "/home/u/.local/bin/karya edit",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("ShellEnv missing %q; got:\n%s", want, script)
		}
	}
	// The managed tool bin and mise shims are session-scoped (config.Env), so they
	// must NOT appear in the global shell integration.
	if strings.Contains(script, p.ToolsBin()) || strings.Contains(script, p.MiseShims()) {
		t.Errorf("ShellEnv must not leak the session toolchain onto global PATH; got:\n%s", script)
	}
	// Guarded PATH edit so repeated evals don't stack duplicates.
	if !strings.Contains(script, "case ") {
		t.Errorf("ShellEnv should guard the PATH edit against duplication; got:\n%s", script)
	}
}

// TestNvimAppNameMatchesConfigDir guards the invariant that ties launch and
// extraction together: NVIM_APPNAME must resolve, under XDG_CONFIG_HOME, to the
// very directory ExtractNvimConfig writes to. If these drift, Neovim reads an
// empty config and the IDE silently loses its editor setup.
func TestNvimAppNameMatchesConfigDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, e := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"} {
		t.Setenv(e, "")
	}
	p := Resolve()
	wantConfig := filepath.Join(home, ".config", NvimAppName)
	if p.NvimConfig() != wantConfig {
		t.Errorf("NvimConfig() = %q, want %q (XDG_CONFIG_HOME/NVIM_APPNAME)", p.NvimConfig(), wantConfig)
	}
}

// TestActivateManagedEnv verifies karya prepends its managed tool dirs to its
// own PATH exactly once (idempotent), preserves existing entries, and exports
// the MISE_* variables so shim-backed tools resolve karya's isolated runtimes.
func TestActivateManagedEnv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, v := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"} {
		t.Setenv(v, "")
	}
	t.Setenv("PATH", "/usr/bin:/bin")

	p := Resolve()
	p.ActivateManagedEnv()

	sep := string(os.PathListSeparator)
	got := os.Getenv("PATH")
	// The legacy shared bin leads, category bins follow, then mise shims.
	if !strings.HasPrefix(got, p.ToolsBin()+sep) {
		t.Fatalf("PATH = %q, want it to start with the managed tool bin %q", got, p.ToolsBin())
	}
	for _, want := range []string{p.ToolCategoryBin("core"), p.MiseShims()} {
		if !strings.Contains(got, want) {
			t.Errorf("PATH missing managed dir %q; got %q", want, got)
		}
	}
	if !strings.Contains(got, "/usr/bin") {
		t.Errorf("PATH dropped existing entries: %q", got)
	}
	if got := os.Getenv("MISE_DATA_DIR"); got != p.MiseData() {
		t.Errorf("MISE_DATA_DIR = %q, want %q", got, p.MiseData())
	}

	// Second activation must not duplicate the managed dirs.
	p.ActivateManagedEnv()
	if n := strings.Count(os.Getenv("PATH"), p.ToolsBin()); n != 1 {
		t.Errorf("ToolsBin appears %d times after re-activation, want 1", n)
	}
}
