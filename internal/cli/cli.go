// Package cli is karya's command dispatcher. Phase 0 uses only the standard
// library; the tree is intentionally simple and every command from docs/PLAN.md
// §4 exists as a stub so the shape is visible and the binary always builds.
// When the tree grows (Phase 1+), migrate to github.com/spf13/cobra.
package cli

import (
	"fmt"
	"os"

	"github.com/drjzlyan/karya/internal/version"
)

// Run dispatches a karya invocation and returns a process exit code.
func Run(args []string) int {
	if len(args) == 0 {
		// Bare `karya` launches/attaches the IDE session for the cwd.
		return cmdDev(nil)
	}

	cmd, rest := args[0], args[1:]
	switch cmd {
	case "-h", "--help", "help":
		usage(os.Stdout)
		return 0
	case "-v", "--version", "version":
		fmt.Println(version.String())
		return 0
	case "dev":
		return cmdDev(rest)
	case "agent":
		return cmdAgent(rest)
	case "edit":
		return notImplemented("edit", "open a file in the editor pane (used as $EDITOR)")
	case "run":
		return notImplemented("run", "run a command in the build/test pane")
	case "new":
		return notImplemented("new", "scaffold a project (python|java|typescript|go|cpp|rust)")
	case "lang":
		return notImplemented("lang", "select languages and runtime versions")
	case "install":
		return notImplemented("install", "set up karya (isolated); extract configs, fetch tools")
	case "update":
		return notImplemented("update", "self-update binary, configs, tools, editor plugins")
	case "uninstall":
		return notImplemented("uninstall", "remove karya entirely (nothing else touched)")
	case "doctor":
		return notImplemented("doctor", "run health checks")
	case "shellenv":
		return notImplemented("shellenv", "print opt-in shell integration")
	case "completion":
		return notImplemented("completion", "print shell completions")
	default:
		fmt.Fprintf(os.Stderr, "karya: unknown command %q\n\n", cmd)
		usage(os.Stderr)
		return 2
	}
}

// cmdDev is the entrypoint for `karya` / `karya dev`. Implemented in Phase 1.
func cmdDev(_ []string) int {
	return notImplemented("dev", "launch or attach the IDE session")
}

// cmdAgent dispatches `karya agent <subcommand>`. Implemented in Phase 2.
func cmdAgent(args []string) int {
	sub := "status"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "status", "switch", "next", "prev", "reset", "prefs", "clear":
		return notImplemented("agent "+sub, "manage the coding agent")
	default:
		fmt.Fprintf(os.Stderr, "karya agent: unknown subcommand %q\n", sub)
		return 2
	}
}

func notImplemented(name, desc string) int {
	fmt.Printf("karya %s — %s\n(not yet implemented; see ROADMAP.md)\n", name, desc)
	return 0
}

func usage(w *os.File) {
	fmt.Fprint(w, `karya — an AI-first, terminal-based IDE in a single binary

Usage:
  karya                     Launch or attach the IDE session for the cwd
  karya dev [name] [path]   Explicit session launch (-a agent, -k kill, -q quit)

  karya agent <cmd>         status | switch | next | prev | reset | prefs | clear
  karya edit <file> [line]  Open a file in the editor pane (used as $EDITOR)
  karya run <cmd...>        Run a command in the build/test pane
  karya new <lang> <name>   Scaffold a project
  karya lang                Select languages and runtime versions

  karya install             Set up karya (isolated, non-destructive)
  karya update [--check]    Self-update binary, configs, tools, plugins
  karya uninstall           Remove karya entirely (nothing else touched)

  karya doctor              Health check
  karya shellenv            Print opt-in shell integration (eval this)
  karya version             Print version / build info
  karya help                Show this help

Docs: docs/PLAN.md · ROADMAP.md · PROGRESS.md
`)
}
