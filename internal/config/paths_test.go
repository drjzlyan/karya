package config

import (
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
