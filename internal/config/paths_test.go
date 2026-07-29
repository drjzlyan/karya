package config

import (
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
		"NVIM_APPNAME=" + AppName,
		"EDITOR=/abs/karya edit",
		"VISUAL=/abs/karya edit",
		"GIT_EDITOR=/abs/karya edit",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("Env missing %q; got:\n%s", want, joined)
		}
	}
}
