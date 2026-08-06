package cli

import (
	"fmt"
	"os"

	"github.com/drjzlyan/karya/internal/ide"
)

// cmdTUI launches karya's single-process TUI IDE for the current directory.
// An optional file argument opens that file in the embedded Neovim editor pane;
// otherwise the first pane is a shell.
//
// karya's own window/pane manager, a unified Ctrl+Space keymap, shell (PTY)
// panes, and the embedded editor. karya views (git, tasks, review) arrive in
// later phases, after which bare `karya` will launch this instead of the tmux
// session.
func cmdTUI(args []string) int {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "karya: cannot determine working directory:", err)
		return 1
	}
	file := ""
	if len(args) > 0 {
		file = args[0]
	}
	if err := ide.Run(dir, file); err != nil {
		fmt.Fprintln(os.Stderr, "karya:", err)
		return 1
	}
	return 0
}
