// Package assets embeds karya's configuration payload (tmux.conf, and later the
// Neovim config) into the binary via go:embed and extracts it to karya-owned
// directories. This is what lets karya ship as a single binary while still
// providing the full tmux + Neovim experience.
package assets

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"
)

//go:embed tmux.conf
var tmuxConf string

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
