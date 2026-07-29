// Package cli is karya's command dispatcher. Phase 0 uses only the standard
// library; the tree is intentionally simple and every command from docs/PLAN.md
// §4 exists as a stub so the shape is visible and the binary always builds.
// When the tree grows (Phase 1+), migrate to github.com/spf13/cobra.
package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/drjzlyan/karya/internal/agent"
	"github.com/drjzlyan/karya/internal/editor"
	"github.com/drjzlyan/karya/internal/session"
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
		return cmdEdit(rest)
	case "run":
		return cmdRun(rest)
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

// cmdDev is the entrypoint for `karya` / `karya dev`.
func cmdDev(args []string) int {
	fs := flag.NewFlagSet("dev", flag.ContinueOnError)
	agentFlag := fs.String("a", "", "coding agent to use (name or 'none')")
	fs.StringVar(agentFlag, "agent", "", "coding agent to use (name or 'none')")
	kill := fs.Bool("k", false, "kill an existing session and recreate it")
	fs.BoolVar(kill, "kill", false, "kill an existing session and recreate it")
	quit := fs.Bool("q", false, "quit (kill) the session cleanly")
	fs.BoolVar(quit, "quit", false, "quit (kill) the session cleanly")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	name := fs.Arg(0)
	workdir := fs.Arg(1)
	if workdir == "" {
		workdir, _ = os.Getwd()
	} else {
		workdir = expandHome(workdir)
	}
	if name == "" {
		name = sanitizeSession(filepath.Base(workdir))
	}

	a, err := newApp()
	if err != nil {
		return fail(err)
	}

	if *quit {
		if err := session.Quit(a.tmux, name); err != nil {
			return fail(err)
		}
		return 0
	}

	if st, err := os.Stat(workdir); err != nil || !st.IsDir() {
		return fail(fmt.Errorf("directory %q does not exist", workdir))
	}

	detected := agent.Detect()
	resolved := agent.Resolve(*agentFlag, detected)
	if err := session.Dev(a.tmux, session.Options{
		Name:     name,
		Workdir:  workdir,
		Agent:    resolved,
		Detected: detected,
		Kill:     *kill,
	}); err != nil {
		return fail(err)
	}
	return 0
}

// cmdEdit opens a file in the editor pane (also used as $EDITOR).
func cmdEdit(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: karya edit <file> [line]")
		return 2
	}
	line := 1
	if len(args) > 1 {
		if n, err := strconv.Atoi(args[1]); err == nil {
			line = n
		}
	}
	a, err := newApp()
	if err != nil {
		return fail(err)
	}
	if err := editor.Edit(a.tmux, args[0], line); err != nil {
		return fail(err)
	}
	return 0
}

// cmdRun sends a command to the build/test pane.
func cmdRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	dir := fs.String("d", "", "run the command in this directory")
	focus := fs.Bool("focus", false, "focus the build/test pane (no command)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	command := strings.Join(fs.Args(), " ")
	if !*focus && command == "" {
		fmt.Fprintln(os.Stderr, "usage: karya run [-d dir] <command> | karya run --focus")
		return 2
	}
	a, err := newApp()
	if err != nil {
		return fail(err)
	}
	if err := editor.Run(a.tmux, command, *dir, *focus); err != nil {
		return fail(err)
	}
	return 0
}

// cmdAgent dispatches `karya agent <subcommand>`. status is implemented in
// Phase 1; switch/next/prev/reset/clear arrive in Phase 2.
func cmdAgent(args []string) int {
	sub := "status"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "status":
		return agentStatus()
	case "switch", "next", "prev", "reset", "prefs", "clear":
		return notImplemented("agent "+sub, "manage the coding agent")
	default:
		fmt.Fprintf(os.Stderr, "karya agent: unknown subcommand %q\n", sub)
		return 2
	}
}

func agentStatus() int {
	detected := agent.Detect()
	if len(detected) == 0 {
		fmt.Println("Available agents: none detected")
		fmt.Printf("Known agents (install any): %s\n", strings.Join(agent.Known, ", "))
		return 0
	}
	fmt.Printf("Available agents: %s\n", strings.Join(detected, ", "))
	return 0
}

// expandHome expands a leading ~ to the user's home directory.
func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

// sanitizeSession makes a string safe as a tmux session name (no dots/colons).
func sanitizeSession(s string) string {
	return strings.NewReplacer(".", "_", ":", "_", " ", "_").Replace(s)
}

func fail(err error) int {
	fmt.Fprintf(os.Stderr, "karya: %v\n", err)
	return 1
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
