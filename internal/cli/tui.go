package cli

import (
	"fmt"
	"os"

	"github.com/drjzlyan/karya/internal/ide"
)

// cmdTUI launches karya's single-process TUI IDE for the current directory.
//
// This is the Phase-1 walking skeleton: karya's own window/pane manager, a
// unified Ctrl+Space keymap, and shell (PTY) panes. The embedded editor and
// karya views arrive in later phases, after which bare `karya` will launch this
// instead of the tmux session.
func cmdTUI(args []string) int {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "karya: cannot determine working directory:", err)
		return 1
	}
	if err := ide.Run(dir); err != nil {
		fmt.Fprintln(os.Stderr, "karya:", err)
		return 1
	}
	return 0
}
