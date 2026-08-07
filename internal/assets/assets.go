// Package assets embeds karya's configuration payload (tmux.conf, and later the
// Neovim config) into the binary via go:embed and extracts it to karya-owned
// directories. This is what lets karya ship as a single binary while still
// providing the full tmux + Neovim experience.
package assets

import (
	"embed"
	"os"
	"path/filepath"
	"strings"
)

//go:embed tmux.conf
var tmuxConf string

//go:embed shell/zshrc shell/bashrc shell/starship.toml
var shellFS embed.FS

//go:embed nvimengine/init.lua
var nvimEngineFS embed.FS

// ExtractNvimEngine writes karya's plugin-free Neovim engine config into dir
// (typically the karya/nvim-engine app-name config dir). The embedded editor
// loads this instead of the user's config, so it gets options + syntax +
// treesitter + native LSP with no plugin bootstrap. Rewritten every call so
// `karya update` refreshes it; idempotent and cheap.
func ExtractNvimEngine(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := nvimEngineFS.ReadFile("nvimengine/init.lua")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "init.lua"), data, 0o644)
}

// shellInitFiles maps an embedded shell asset to its extracted filename. zshrc
// lands as ".zshrc" so a ZDOTDIR pointing at the dir loads it; bashrc keeps its
// name (referenced explicitly via --rcfile); starship.toml is the pinned prompt
// config.
var shellInitFiles = map[string]string{
	"shell/zshrc":         ".zshrc",
	"shell/bashrc":        "bashrc",
	"shell/starship.toml": "starship.toml",
}

// ExtractTmuxConf writes the embedded tmux config to confPath, substituting the
// placeholders so keybindings invoke the real karya binary and reload points at
// the extracted file. It always rewrites the file so `karya update` refreshes it.
func ExtractTmuxConf(confPath, karyaBin string) error {
	if err := os.MkdirAll(filepath.Dir(confPath), 0o755); err != nil {
		return err
	}
	content := strings.NewReplacer(
		"__KARYA_BIN__", karyaBin,
		"__KARYA_CONF__", confPath,
	).Replace(tmuxConf)
	return os.WriteFile(confPath, []byte(content), 0o644)
}

// ExtractShellInit writes karya's shell startup files (zsh .zshrc, bash rcfile,
// starship.toml) into dir. The pane shell (`karya shell`) points ZDOTDIR/--rcfile
// here so it can source the user's real rc and then layer karya's starship prompt
// — all inside karya-owned files, so the user's own rc is never touched. It always
// rewrites the files so `karya update` refreshes them; idempotent and cheap.
func ExtractShellInit(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for src, name := range shellInitFiles {
		data, err := shellFS.ReadFile(src)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
