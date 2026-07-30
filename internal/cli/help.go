package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/drjzlyan/karya/internal/assets"
)

// commandHelp is the per-command reference surfaced by `karya help <command>`.
// It is richer than the flag usage each command prints: a synopsis, the concrete
// syntax, and a few lines of guidance. The full narrative lives in the embedded
// docs (`karya docs keymaps` / `karya docs tutorial`).
type commandHelp struct {
	syntax  string
	summary string
	details []string
}

// helpByCommand maps a command name to its help entry. Keep in step with the
// command switch in Run and the usage() summary.
var helpByCommand = map[string]commandHelp{
	"dev": {
		syntax:  "karya dev [name] [path]",
		summary: "Launch or attach the isolated IDE session for a project.",
		details: []string{
			"Bare `karya` is shorthand for `karya dev` on the current directory.",
			"Flags: -a <agent> pick the coding agent (`none` for a plain shell),",
			"       -k kill and recreate the session, -q quit it cleanly.",
		},
	},
	"agent": {
		syntax:  "karya agent <status|switch|next|prev|reset|prefs|clear>",
		summary: "Inspect and switch the session's coding agent.",
		details: []string{
			"status  show the current/available agents and saved preference",
			"switch  interactive picker (in session); next/prev cycle agents",
			"reset   rebuild the pane layout, preserving the editor",
			"prefs   list per-project preferences; clear removes this project's",
		},
	},
	"edit": {
		syntax:  "karya edit <file> [line]",
		summary: "Open a file in the session's editor pane; used as $EDITOR.",
	},
	"run": {
		syntax:  "karya run [-d dir] <command> | karya run --focus",
		summary: "Run a command in the build/test pane (or just focus it).",
	},
	"new": {
		syntax:  "karya new <lang> <name> [dir]",
		summary: "Scaffold a new project and open it in an IDE session.",
		details: []string{
			"Languages: python, java, typescript, go, cpp, rust.",
			"Runs `git init` and, from inside a session, switches to the new one.",
		},
	},
	"lang": {
		syntax:  "karya lang <list|add|remove|all>",
		summary: "Select project languages and install their runtimes + tools.",
		details: []string{
			"add <lang> [versions]  install a language toolchain into karya's prefix",
			"remove <lang>          drop a language; list shows the current selection",
		},
	},
	"install": {
		syntax:  "karya install",
		summary: "Set up karya: extract configs, install tools, sync plugins.",
		details: []string{"Isolated and non-destructive — it touches only karya-owned dirs."},
	},
	"update": {
		syntax:  "karya update [--check]",
		summary: "Self-update the binary, configs, tools and plugins.",
		details: []string{"--check reports whether a newer release exists without applying it."},
	},
	"uninstall": {
		syntax:  "karya uninstall [-y]",
		summary: "Remove karya entirely; nothing else on the system is touched.",
	},
	"doctor": {
		syntax:  "karya doctor",
		summary: "Check tools, versions, and isolation for problems.",
	},
	"shellenv": {
		syntax:  "karya shellenv",
		summary: "Print opt-in shell integration to eval in your rc file.",
		details: []string{`Add: eval "$(karya shellenv)"`},
	},
	"docs": {
		syntax:  "karya docs [topic]",
		summary: "Read the embedded documentation offline (no topic lists them).",
	},
	"help": {
		syntax:  "karya help [command]",
		summary: "Show general help, or detailed help for one command.",
		details: []string{"`karya help topics` lists every command you can ask about."},
	},
	"version": {
		syntax:  "karya version",
		summary: "Print the version and build information.",
	},
}

// cmdHelp implements `karya help [command|topics]`.
func cmdHelp(args []string) int {
	if len(args) == 0 {
		usage(os.Stdout)
		return 0
	}
	if args[0] == "topics" {
		printHelpTopics(os.Stdout)
		return 0
	}
	return printCommandHelp(os.Stdout, args[0])
}

// printCommandHelp writes the detailed help for one command, or an error listing
// the available commands. It returns the process exit code.
func printCommandHelp(w io.Writer, name string) int {
	h, ok := helpByCommand[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "karya help: no such command %q\n", name)
		printHelpTopics(os.Stderr)
		return 2
	}
	fmt.Fprintf(w, "karya %s — %s\n\n", name, h.summary)
	fmt.Fprintf(w, "Usage:\n  %s\n", h.syntax)
	if len(h.details) > 0 {
		fmt.Fprintln(w)
		for _, d := range h.details {
			fmt.Fprintf(w, "  %s\n", d)
		}
	}
	fmt.Fprintf(w, "\nFull reference: karya docs keymaps · Tutorial: karya docs tutorial\n")
	return 0
}

// printHelpTopics lists the commands `karya help <command>` can describe.
func printHelpTopics(w io.Writer) {
	fmt.Fprintln(w, "Commands you can get help for (karya help <command>):")
	for _, name := range helpCommandNames() {
		fmt.Fprintf(w, "  %-10s %s\n", name, helpByCommand[name].summary)
	}
}

// helpCommandNames returns the help command names in sorted order.
func helpCommandNames() []string {
	names := make([]string, 0, len(helpByCommand))
	for name := range helpByCommand {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// cmdDocs implements `karya docs [topic]` — offline access to the embedded docs.
func cmdDocs(args []string) int {
	if len(args) == 0 {
		printDocTopics(os.Stdout)
		return 0
	}
	topic := args[0]
	content, ok := assets.Doc(topic)
	if !ok {
		fmt.Fprintf(os.Stderr, "karya docs: no such topic %q\n", topic)
		printDocTopics(os.Stderr)
		return 2
	}
	return page(content)
}

// printDocTopics lists the embedded documentation topics.
func printDocTopics(w io.Writer) {
	topics := assets.DocTopics()
	if len(topics) == 0 {
		fmt.Fprintln(w, "No documentation is embedded in this build.")
		return
	}
	fmt.Fprintln(w, "Documentation topics (karya docs <topic>):")
	for _, t := range topics {
		fmt.Fprintf(w, "  %s\n", t)
	}
}

// page displays content through the user's pager when stdout is a terminal,
// falling back to a plain write otherwise (pipes, redirects, no pager). It never
// fails the command over a display problem — the content is always printed.
func page(content string) int {
	if pager := pagerCommand(); pager != nil && isTerminal(os.Stdout) {
		pager.Stdin = strings.NewReader(content)
		pager.Stdout = os.Stdout
		pager.Stderr = os.Stderr
		if pager.Run() == nil {
			return 0
		}
	}
	fmt.Fprint(os.Stdout, content)
	if !strings.HasSuffix(content, "\n") {
		fmt.Fprintln(os.Stdout)
	}
	return 0
}

// pagerCommand builds the pager command from $PAGER, falling back to `less -R`
// then `more`, or nil when none is available.
func pagerCommand() *exec.Cmd {
	if p := strings.TrimSpace(os.Getenv("PAGER")); p != "" {
		fields := strings.Fields(p)
		if _, err := exec.LookPath(fields[0]); err == nil {
			return exec.Command(fields[0], fields[1:]...) //nolint:gosec // user-configured pager
		}
	}
	if path, err := exec.LookPath("less"); err == nil {
		return exec.Command(path, "-R")
	}
	if path, err := exec.LookPath("more"); err == nil {
		return exec.Command(path)
	}
	return nil
}

// isTerminal reports whether f is a character device (a terminal), so we only
// invoke a pager for interactive output.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
