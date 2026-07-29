//go:build integration

// These integration tests drive the real `nvim` binary. They are excluded from
// the default `go test` run (gated behind the `integration` build tag) and are
// what prove karya's Neovim isolation guarantee end-to-end against Neovim's own
// path resolution. CI installs nvim and runs them with -tags=integration.
package editor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/drjzlyan/karya/internal/assets"
	"github.com/drjzlyan/karya/internal/config"
)

// TestNvimIsolation proves the defining Phase 3 guarantee: launched with karya's
// NVIM_APPNAME, Neovim resolves every stdpath under the karya prefix and never
// reads or writes the user's own ~/.config/nvim. It sandboxes HOME + all XDG base
// dirs, extracts the embedded config, then asks a real nvim where its dirs are.
func TestNvimIsolation(t *testing.T) {
	nvim, err := exec.LookPath("nvim")
	if err != nil {
		t.Skip("nvim not installed")
	}

	home := t.TempDir()
	xdgConfig := filepath.Join(home, ".config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdgConfig)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	p := config.Resolve()
	if err := assets.ExtractNvimConfig(p.NvimConfig()); err != nil {
		t.Fatalf("ExtractNvimConfig: %v", err)
	}

	// Ask Neovim where its dirs are. `-u NONE` skips loading init.lua (so no plugin
	// network fetch), while stdpath resolution still honours NVIM_APPNAME + XDG.
	lua := `io.write(
		"CONFIG="..vim.fn.stdpath("config").."\n"..
		"DATA="..vim.fn.stdpath("data").."\n"..
		"STATE="..vim.fn.stdpath("state").."\n"..
		"CACHE="..vim.fn.stdpath("cache").."\n")`
	cmd := exec.Command(nvim, "--headless", "-u", "NONE", "-i", "NONE", "+lua "+lua, "+qa")
	cmd.Env = append(os.Environ(), "NVIM_APPNAME="+config.NvimAppName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run nvim: %v\n%s", err, out)
	}

	paths := parseKV(string(out))
	want := map[string]string{
		"CONFIG": p.NvimConfig(),
		"DATA":   filepath.Join(p.Data, "nvim"),
		"STATE":  filepath.Join(p.State, "nvim"),
		"CACHE":  filepath.Join(p.Cache, "nvim"),
	}
	for k, wantPath := range want {
		if got := paths[k]; got != wantPath {
			t.Errorf("stdpath(%s) = %q, want %q (must live under the karya prefix)", strings.ToLower(k), got, wantPath)
		}
	}

	// The user's own Neovim config dir must never be created or touched.
	userNvim := filepath.Join(xdgConfig, "nvim")
	if _, err := os.Stat(userNvim); !os.IsNotExist(err) {
		t.Errorf("user config dir %q exists/was touched (err=%v); isolation violated", userNvim, err)
	}

	// And Neovim reads karya's extracted config: init.lua sits at stdpath(config).
	if _, err := os.Stat(filepath.Join(paths["CONFIG"], "init.lua")); err != nil {
		t.Errorf("extracted init.lua missing at stdpath(config): %v", err)
	}
}

// parseKV parses "KEY=value" lines into a map, ignoring blanks.
func parseKV(s string) map[string]string {
	m := make(map[string]string)
	for _, line := range strings.Split(s, "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
			m[k] = v
		}
	}
	return m
}
