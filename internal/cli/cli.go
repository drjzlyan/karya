// Package cli is karya's command dispatcher. Phase 0 uses only the standard
// library; the tree is intentionally simple and every command from PLAN.md
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
	"github.com/drjzlyan/karya/internal/project"
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
		return cmdNew(rest)
	case "lang":
		return cmdLang(rest)
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

	// Agent resolution: flag → saved per-project preference → single detected →
	// interactive picker. The chosen agent is saved so it persists next launch.
	detected := agent.Detect()
	explicit := *agentFlag
	if explicit == "" {
		explicit = a.prefs.Get("agent." + workdir)
	}
	resolved := agent.Resolve(explicit, detected)
	if resolved != "" {
		_ = a.prefs.Set("agent."+workdir, resolved)
	}
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

// cmdNew scaffolds a new project (`karya new <lang> <name> [dir]`). It also
// accepts the "lang:name" form used by the tmux `Ctrl-a P` command-prompt.
// After scaffolding it runs git init and, when invoked from inside a karya
// session, opens the new project in its own IDE session and switches to it.
func cmdNew(args []string) int {
	lang, name, parent := parseNewArgs(args)
	if lang == "" || name == "" {
		fmt.Fprintf(os.Stderr, "usage: karya new <lang> <name> [dir]\nlanguages: %s\n",
			strings.Join(project.Languages, ", "))
		return 2
	}

	spec, err := project.NewSpec(lang, name)
	if err != nil {
		return fail(err)
	}

	a, err := newApp()
	if err != nil {
		return fail(err)
	}

	// Resolve the parent directory: an explicit third arg wins; otherwise fall
	// back to the current session's workdir, then the cwd.
	if parent == "" {
		parent = sessionWorkdir(a)
	}
	if parent == "" {
		parent, _ = os.Getwd()
	}
	parent = expandHome(parent)

	dir, err := project.Scaffold(parent, spec)
	if err != nil {
		return fail(err)
	}
	if err := project.GitInit(dir); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}
	fmt.Printf("Created %s project: %s\n", spec.Lang, dir)

	if !openInSession(a, spec.Basename, dir) {
		fmt.Printf("Launch it with: karya dev %s %s\n", spec.Basename, dir)
	}
	return 0
}

// parseNewArgs accepts both `karya new <lang> <name> [dir]` and the single
// "lang:name" token form. It returns empty strings when the input is unusable.
func parseNewArgs(args []string) (lang, name, parent string) {
	switch {
	case len(args) == 1 && strings.Contains(args[0], ":"):
		l, n, _ := strings.Cut(args[0], ":")
		return strings.TrimSpace(l), strings.TrimSpace(n), ""
	case len(args) >= 2:
		if len(args) >= 3 {
			parent = args[2]
		}
		return args[0], args[1], parent
	default:
		return "", "", ""
	}
}

// sessionWorkdir returns the @ide_workdir of the karya session we are inside, or
// "" when not in one.
func sessionWorkdir(a *app) string {
	if os.Getenv("TMUX") == "" {
		return ""
	}
	name, err := a.tmux.Output("display-message", "-p", "#{session_name}")
	if err != nil || name == "" {
		return ""
	}
	wd, err := a.tmux.Output("show-option", "-t", name, "-v", "@ide_workdir")
	if err != nil {
		return ""
	}
	return wd
}

// openInSession opens dir as a karya IDE session named after the project and
// switches the current client to it. It is a no-op (returning false) when not
// inside a karya session. An existing session with the same name is reused.
func openInSession(a *app, name, dir string) bool {
	if os.Getenv("TMUX") == "" {
		return false
	}
	name = sanitizeSession(name)
	if a.tmux.HasSession(name) {
		_ = a.tmux.Run("switch-client", "-t", name+":dev")
		return true
	}
	detected := agent.Detect()
	resolved := agent.Resolve(a.prefs.Get("agent."+dir), detected)
	if resolved != "" {
		_ = a.prefs.Set("agent."+dir, resolved)
	}
	if err := session.Build(a.tmux, session.Options{
		Name:     name,
		Workdir:  dir,
		Agent:    resolved,
		Detected: detected,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not open session: %v\n", err)
		return false
	}
	_ = a.tmux.Run("switch-client", "-t", name+":dev")
	fmt.Printf("Opened session %q\n", name)
	return true
}

// cmdAgent dispatches `karya agent <subcommand>`. In-session subcommands
// (switch/next/prev/reset/clear) operate on the current tmux session's @ide_*
// state via agent.Manager; status and prefs also work from a plain terminal.
func cmdAgent(args []string) int {
	sub := "status"
	if len(args) > 0 {
		sub = args[0]
	}
	rest := args[1:]

	a, err := newApp()
	if err != nil {
		return fail(err)
	}

	switch sub {
	case "status":
		return agentStatus(a)
	case "prefs":
		return agentPrefs(a)
	case "switch", "switch-to", "next", "prev", "reset", "clear":
		return agentInSession(a, sub, rest)
	default:
		fmt.Fprintf(os.Stderr, "karya agent: unknown subcommand %q\n", sub)
		return 2
	}
}

// agentManager builds a Manager for the current tmux session, or reports that we
// are not inside one.
func agentManager(a *app) (*agent.Manager, bool) {
	name, err := a.tmux.Output("display-message", "-p", "#{session_name}")
	if err != nil || name == "" {
		return nil, false
	}
	return agent.NewManager(a.tmux, a.prefs, name, a.bin), true
}

// agentInSession runs an in-session agent subcommand, requiring a karya session.
func agentInSession(a *app, sub string, rest []string) int {
	m, ok := agentManager(a)
	if !ok {
		fmt.Fprintln(os.Stderr, "karya agent: not in a karya session")
		return 1
	}
	var err error
	switch sub {
	case "switch":
		err = m.SwitchInteractive()
	case "switch-to":
		name := agent.None
		if len(rest) > 0 && rest[0] != "" {
			name = rest[0]
		}
		err = m.SwitchTo(name)
	case "next":
		err = m.Next()
	case "prev":
		err = m.Prev()
	case "reset":
		err = m.Reset()
	case "clear":
		err = m.ClearPref()
	}
	if err != nil {
		return fail(err)
	}
	return 0
}

// agentStatus prints the current agent and availability. Inside a session it
// shows full @ide_* state; from a terminal it lists detected agents.
func agentStatus(a *app) int {
	if m, ok := agentManager(a); ok {
		fmt.Println(m.StatusText())
		return 0
	}
	detected := agent.Detect()
	if len(detected) == 0 {
		fmt.Println("Available agents: none detected")
		fmt.Printf("Known agents (install any): %s\n", strings.Join(agent.Known, ", "))
		return 0
	}
	fmt.Printf("Available agents: %s\n", strings.Join(detected, ", "))
	return 0
}

// agentPrefs prints the stored per-project preferences.
func agentPrefs(a *app) int {
	entries := a.prefs.Entries()
	if len(entries) == 0 {
		fmt.Printf("No preferences at %s\n", a.prefs.Path())
		return 0
	}
	fmt.Printf("karya preferences (%s):\n", a.prefs.Path())
	for _, e := range entries {
		fmt.Printf("  %s\n", e)
	}
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
  karya new <lang> <name>   Scaffold a project (python|java|typescript|go|cpp|rust)
  karya lang <cmd>          list | add <lang> [versions] | remove <lang> | all

  karya install             Set up karya (isolated, non-destructive)
  karya update [--check]    Self-update binary, configs, tools, plugins
  karya uninstall           Remove karya entirely (nothing else touched)

  karya doctor              Health check
  karya shellenv            Print opt-in shell integration (eval this)
  karya version             Print version / build info
  karya help                Show this help

Docs: docs/tutorial.md · docs/keymaps.md
`)
}
