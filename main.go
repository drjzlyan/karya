// Command karya is an AI-first, terminal-based IDE delivered as a single binary.
// It orchestrates Neovim, tmux, and a coding agent into one cohesive IDE while
// keeping all of its state isolated from the user's existing settings.
//
// See PLAN.md for the architecture and the isolation model that governs it.
package main

import (
	"os"

	"github.com/drjzlyan/karya/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
