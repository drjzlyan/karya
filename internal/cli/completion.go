package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/drjzlyan/karya/internal/assets"
	"github.com/drjzlyan/karya/internal/project"
)

// compCmd models one command for shell completion: its name, a short
// description (shown by zsh/fish), the static values that follow it, and whether
// it takes a file argument (edit/run).
type compCmd struct {
	name    string
	desc    string
	sub     []string
	isFiles bool
}

// completionModel is the single source of truth the bash/zsh/fish generators
// render from. Command order mirrors the dispatcher in Run; descriptions reuse
// the per-command help summaries so completion stays in step with `karya help`.
func completionModel() []compCmd {
	order := []string{
		"dev", "agent", "edit", "run", "new", "lang", "profile", "tool",
		"install", "update", "uninstall", "doctor", "shellenv",
		"completion", "docs", "tutorial", "help", "version",
	}
	sub := map[string][]string{
		"agent":      {"status", "switch", "next", "prev", "reset", "prefs", "clear"},
		"lang":       {"list", "add", "remove", "all"},
		"profile":    {"list", "install"},
		"tool":       {"list", "update"},
		"new":        project.Languages,
		"docs":       assets.DocTopics(),
		"help":       append([]string{"topics"}, helpCommandNames()...),
		"completion": completionShells,
		"tutorial":   {"list"},
	}
	model := make([]compCmd, 0, len(order))
	for _, name := range order {
		c := compCmd{name: name, sub: sub[name]}
		if h, ok := helpByCommand[name]; ok {
			c.desc = h.summary
		}
		if name == "edit" || name == "run" {
			c.isFiles = true
		}
		model = append(model, c)
	}
	return model
}

// completionShells are the shells `karya completion` can generate for.
var completionShells = []string{"bash", "zsh", "fish"}

// cmdCompletion implements `karya completion <bash|zsh|fish>`.
func cmdCompletion(args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: karya completion <%s>\n", strings.Join(completionShells, "|"))
		return 2
	}
	shell := args[0]
	model := completionModel()
	switch shell {
	case "bash":
		writeBashCompletion(os.Stdout, model)
	case "zsh":
		writeZshCompletion(os.Stdout, model)
	case "fish":
		writeFishCompletion(os.Stdout, model)
	default:
		fmt.Fprintf(os.Stderr, "karya completion: unsupported shell %q (want %s)\n",
			shell, strings.Join(completionShells, ", "))
		return 2
	}
	return 0
}

// commandNames returns just the top-level command names in dispatcher order.
func commandNames(model []compCmd) []string {
	names := make([]string, len(model))
	for i, c := range model {
		names[i] = c.name
	}
	return names
}

// writeBashCompletion renders a bash completion script. Top-level completes the
// command; the second word completes a command's static values; edit/run
// complete filenames.
func writeBashCompletion(w io.Writer, model []compCmd) {
	fmt.Fprintf(w, "# bash completion for karya — eval \"$(karya completion bash)\"\n")
	fmt.Fprintf(w, "_karya() {\n")
	fmt.Fprintf(w, "    local cur cmd\n")
	fmt.Fprintf(w, "    cur=\"${COMP_WORDS[COMP_CWORD]}\"\n")
	fmt.Fprintf(w, "    cmd=\"${COMP_WORDS[1]}\"\n")
	fmt.Fprintf(w, "    if [ \"$COMP_CWORD\" -eq 1 ]; then\n")
	fmt.Fprintf(w, "        COMPREPLY=( $(compgen -W %q -- \"$cur\") )\n", strings.Join(commandNames(model), " "))
	fmt.Fprintf(w, "        return 0\n    fi\n")
	fmt.Fprintf(w, "    case \"$cmd\" in\n")
	fmt.Fprintf(w, "        edit|run) COMPREPLY=( $(compgen -f -- \"$cur\") ); return 0 ;;\n")
	fmt.Fprintf(w, "    esac\n")
	fmt.Fprintf(w, "    if [ \"$COMP_CWORD\" -eq 2 ]; then\n")
	fmt.Fprintf(w, "        case \"$cmd\" in\n")
	for _, c := range model {
		if len(c.sub) == 0 {
			continue
		}
		fmt.Fprintf(w, "            %s) COMPREPLY=( $(compgen -W %q -- \"$cur\") ) ;;\n",
			c.name, strings.Join(c.sub, " "))
	}
	fmt.Fprintf(w, "        esac\n    fi\n")
	fmt.Fprintf(w, "    return 0\n}\n")
	fmt.Fprintf(w, "complete -F _karya karya\n")
}

// writeZshCompletion renders a native zsh completion script.
func writeZshCompletion(w io.Writer, model []compCmd) {
	fmt.Fprintf(w, "#compdef karya\n")
	fmt.Fprintf(w, "# zsh completion for karya — eval \"$(karya completion zsh)\"\n")
	fmt.Fprintf(w, "_karya() {\n")
	fmt.Fprintf(w, "    local -a commands\n")
	fmt.Fprintf(w, "    commands=(\n")
	for _, c := range model {
		fmt.Fprintf(w, "        %s\n", zshDescribe(c.name, c.desc))
	}
	fmt.Fprintf(w, "    )\n")
	fmt.Fprintf(w, "    if (( CURRENT == 2 )); then\n")
	fmt.Fprintf(w, "        _describe 'karya command' commands\n")
	fmt.Fprintf(w, "        return\n    fi\n")
	fmt.Fprintf(w, "    case ${words[2]} in\n")
	fmt.Fprintf(w, "        edit|run) _files ;;\n")
	for _, c := range model {
		if len(c.sub) == 0 {
			continue
		}
		fmt.Fprintf(w, "        %s) (( CURRENT == 3 )) && compadd %s ;;\n",
			c.name, strings.Join(c.sub, " "))
	}
	fmt.Fprintf(w, "    esac\n}\n")
	fmt.Fprintf(w, "compdef _karya karya\n")
}

// zshDescribe formats a "name:description" entry for zsh's _describe, quoting it
// safely. When there is no description it emits the bare name.
func zshDescribe(name, desc string) string {
	if desc == "" {
		return "'" + name + "'"
	}
	// _describe splits on the first colon; strip colons from the description.
	desc = strings.ReplaceAll(desc, ":", " -")
	desc = strings.ReplaceAll(desc, "'", "")
	return "'" + name + ":" + desc + "'"
}

// writeFishCompletion renders a fish completion script.
func writeFishCompletion(w io.Writer, model []compCmd) {
	fmt.Fprintf(w, "# fish completion for karya — karya completion fish | source\n")
	fmt.Fprintf(w, "complete -c karya -f\n")
	for _, c := range model {
		fmt.Fprintf(w, "complete -c karya -n __fish_use_subcommand -a %s -d %s\n",
			fishQuote(c.name), fishQuote(c.desc))
	}
	for _, c := range model {
		if c.isFiles {
			fmt.Fprintf(w, "complete -c karya -n '__fish_seen_subcommand_from %s' -F\n", c.name)
			continue
		}
		if len(c.sub) == 0 {
			continue
		}
		fmt.Fprintf(w, "complete -c karya -n '__fish_seen_subcommand_from %s' -a %s\n",
			c.name, fishQuote(strings.Join(c.sub, " ")))
	}
}

// fishQuote single-quotes a value for fish, escaping embedded quotes.
func fishQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `\'`) + "'"
}
