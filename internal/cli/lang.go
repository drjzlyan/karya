package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/drjzlyan/karya/internal/lang"
	"github.com/drjzlyan/karya/internal/toolreg"
	"github.com/drjzlyan/karya/internal/tools"
)

// cmdLang dispatches `karya lang [list|add|remove|all]`. With no subcommand it
// opens the interactive selector. Any mutation re-applies the selection:
// languages.local is rewritten, the isolated mise config is regenerated, and
// runtimes + LSP/formatter/adapter tools are installed into the karya prefix —
// never touching Homebrew or the user's global mise (archive/v0/PLAN.md §6.4).
func cmdLang(args []string) int {
	a, err := newApp()
	if err != nil {
		return fail(err)
	}
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	rest := args[1:]

	switch sub {
	case "", "select":
		return langInteractive(a)
	case "list":
		return langList(a)
	case "add":
		return langAdd(a, rest)
	case "remove", "rm":
		return langRemove(a, rest)
	case "all":
		return langAll(a)
	default:
		fmt.Fprintf(os.Stderr, "usage: karya lang [list|add <lang> [versions]|remove <lang>|all]\n")
		return 2
	}
}

// langList prints the current selection.
func langList(a *app) int {
	sel, err := loadSelection(a)
	if err != nil {
		return fail(err)
	}
	if sel.Empty() {
		fmt.Println("No languages selected. Run `karya lang` to choose.")
		return 0
	}
	fmt.Printf("Selected languages (%s):\n", a.paths.LanguagesFile())
	for _, l := range sel.Langs() {
		fmt.Printf("  %-12s %s\n", l, strings.Join(sel.Versions(l), ", "))
	}
	return 0
}

// langAdd sets a language's versions and applies. Versions may be given as a
// trailing comma- or space-separated list; when omitted karya offers a picker.
func langAdd(a *app, args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: karya lang add <lang> [versions]\nlanguages: %s\n",
			strings.Join(lang.Names(), ", "))
		return 2
	}
	l, ok := lang.Find(args[0])
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown language %q (supported: %s)\n", args[0], strings.Join(lang.Names(), ", "))
		return 2
	}
	sel, err := loadSelection(a)
	if err != nil {
		return fail(err)
	}

	var versions []string
	switch {
	case l.System:
		versions = []string{l.Fallback} // system toolchain: no version to pick
	case len(args) > 1:
		versions = splitVersions(args[1:])
	default:
		versions = pickVersions(a, l, sel.Versions(l.Name))
	}
	if len(versions) == 0 {
		fmt.Println("No versions chosen; nothing changed.")
		return 0
	}
	sel.Set(l.Name, versions)
	return applySelection(a, sel)
}

// langRemove drops a language and applies.
func langRemove(a *app, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: karya lang remove <lang>")
		return 2
	}
	l, ok := lang.Find(args[0])
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown language %q\n", args[0])
		return 2
	}
	sel, err := loadSelection(a)
	if err != nil {
		return fail(err)
	}
	if !sel.Has(l.Name) {
		fmt.Printf("%s is not selected.\n", l.Display)
		return 0
	}
	sel.Remove(l.Name)
	return applySelection(a, sel)
}

// langAll selects every language at its latest-stable fallback and applies.
func langAll(a *app) int {
	sel := lang.NewSelection()
	for _, l := range lang.Catalog {
		sel.Set(l.Name, []string{l.Fallback})
	}
	return applySelection(a, sel)
}

// langInteractive runs the menu loop then applies the resulting selection.
func langInteractive(a *app) int {
	sel, err := loadSelection(a)
	if err != nil {
		return fail(err)
	}
	selectLanguages(a, sel)
	return applySelection(a, sel)
}

// selectLanguages runs the interactive menu loop against sel, mutating it in
// place: choose a language to configure, r<N> to remove, a to select all, n to
// clear, Enter to finish. It does not persist or apply — callers do that so the
// picker can be reused (e.g. inline during `karya install`).
func selectLanguages(a *app, sel *lang.Selection) {
	in := bufio.NewReader(os.Stdin)
	for {
		printLangMenu(sel)
		fmt.Print("  > ")
		line, err := in.ReadString('\n')
		choice := strings.TrimSpace(line)
		if choice == "" {
			break
		}
		switch {
		case choice == "a" || choice == "A":
			for _, l := range lang.Catalog {
				sel.Set(l.Name, []string{l.Fallback})
			}
		case choice == "n" || choice == "N":
			sel.Clear()
		case strings.HasPrefix(strings.ToLower(choice), "r"):
			if n, e := strconv.Atoi(choice[1:]); e == nil && n >= 1 && n <= len(lang.Catalog) {
				sel.Remove(lang.Catalog[n-1].Name)
			}
		default:
			if n, e := strconv.Atoi(choice); e == nil && n >= 1 && n <= len(lang.Catalog) {
				l := lang.Catalog[n-1]
				if l.System {
					sel.Set(l.Name, []string{l.Fallback})
				} else {
					sel.Set(l.Name, pickVersions(a, l, sel.Versions(l.Name)))
				}
			} else {
				fmt.Println("  Invalid choice (use a number, r<N>, a, n, or Enter).")
			}
		}
		if err != nil {
			break // EOF on stdin
		}
	}
}

// printLangMenu draws the selection state.
func printLangMenu(sel *lang.Selection) {
	fmt.Println()
	fmt.Println("  Language & version setup")
	fmt.Println("  Always available: JSON, YAML, TOML, Bash, Lua, Markdown")
	fmt.Println("  <N> configure   r<N> remove   a all   n clear   Enter finish")
	fmt.Println()
	for i, l := range lang.Catalog {
		mark := " "
		info := "-"
		if sel.Has(l.Name) {
			mark = "x"
			info = strings.Join(sel.Versions(l.Name), ", ")
		}
		fmt.Printf("  [%s] %d. %-14s %s\n", mark, i+1, l.Display, info)
	}
}

// pickVersions offers the available versions for a language and reads a choice
// (numbers, comma list, or exact versions). Enter keeps the current selection or
// falls back to the language default.
func pickVersions(a *app, l lang.Language, current []string) []string {
	available := lang.AvailableVersions(langLister(a), l)
	fmt.Printf("\n  %s versions (via mise):\n", l.Display)
	for i, v := range available {
		mark := " "
		for _, c := range current {
			if c == v {
				mark = "*"
			}
		}
		fmt.Printf("  %s %2d. %s\n", mark, i+1, v)
	}
	def := strings.Join(current, ",")
	if def == "" {
		def = l.Fallback
	}
	fmt.Printf("  Pick number(s) (e.g. 1,3), exact versions, or Enter for %s\n  > ", def)

	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	choice := strings.TrimSpace(line)
	if choice == "" {
		return splitVersions([]string{def})
	}
	// Numeric selections index into the offered list; anything else is taken
	// verbatim as explicit version specifiers.
	var out []string
	numeric := true
	for _, tok := range strings.FieldsFunc(choice, func(r rune) bool { return r == ',' || r == ' ' }) {
		if n, err := strconv.Atoi(tok); err == nil && n >= 1 && n <= len(available) {
			out = append(out, available[n-1])
		} else {
			numeric = false
			break
		}
	}
	if numeric && len(out) > 0 {
		return out
	}
	return splitVersions([]string{choice})
}

// applySelection persists the selection, regenerates the isolated mise config,
// installs runtimes, and installs the language tools.
func applySelection(a *app, sel *lang.Selection) int {
	if err := lang.SaveSelection(a.paths.LanguagesFile(), sel); err != nil {
		return fail(err)
	}
	fmt.Printf("Saved selection to %s\n", a.paths.LanguagesFile())

	// Provision the isolated mise on demand so `karya lang` works on a machine
	// that never ran `karya install`. Best-effort: stay usable offline.
	if _, err := tools.EnsureMise(a.paths, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}

	// Regenerate the isolated mise config and install the selected runtimes.
	rm := lang.RuntimeManager{
		MiseConfigPath: a.paths.MiseConfig(),
		GoPath:         filepath.Join(a.paths.Data, "go"),
		CargoHome:      filepath.Join(a.paths.Data, "cargo"),
		Env:            append(os.Environ(), a.env...),
		Out:            os.Stdout,
		ErrOut:         os.Stderr,
	}
	if ran, err := rm.Ensure(sel, miseToolsFor(a.reg, sel)); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	} else if !ran {
		fmt.Println("Could not provision mise — runtimes not installed. Re-run `karya lang all` when back online.")
	}

	_ = a.paths.EnsureDirs()
	// Install the always-on servers plus each selected language's tooling from the
	// registry, into karya's category-based prefix. Dependencies (runtimes, uv)
	// are pulled in and skipped when already present.
	ids := toolreg.AlwaysOnIDs()
	for _, l := range sel.Langs() {
		ids = append(ids, a.reg.LanguageIDs(l)...)
	}
	results := a.installToolIDs(ids)
	a.refreshToolManifest()
	fmt.Printf("Tools: %s\n", tools.Summarize(results))
	return 0
}

// loadSelection reads the current selection from the karya prefix.
func loadSelection(a *app) (*lang.Selection, error) {
	return lang.LoadSelection(a.paths.LanguagesFile())
}

// miseToolsFor returns every mise-provisioned tool to declare in karya's
// generated config: the managed baseline (core CLI, documentation, always-on
// servers, plus the tmux/Neovim/uv infra) always, and each selected language's
// mise tools. Declaring them — not merely installing them — is what gives their
// shims a resolvable version: a `mise install` without a config entry produces a
// shim that errors with "No version is set". Runtimes are excluded here; they are
// declared from the language selection's [tools].
func miseToolsFor(reg *toolreg.Registry, sel *lang.Selection) []lang.MiseTool {
	var out []lang.MiseTool
	seen := map[string]bool{}
	add := func(t toolreg.Tool) {
		if t.Method != toolreg.MethodMise || t.Category == toolreg.Runtime || t.Pkg == "" || seen[t.Pkg] {
			return
		}
		seen[t.Pkg] = true
		out = append(out, lang.MiseTool{Key: t.Pkg, Version: t.Version})
	}
	for _, t := range reg.All() {
		switch t.Location.Kind {
		case toolreg.LocCore, toolreg.LocDocs:
			add(t)
		case toolreg.LocLang:
			if sel != nil && sel.Has(t.Location.Lang) {
				add(t)
			}
		}
	}
	return out
}

// langLister returns a mise-backed version lister pinned to karya's isolated
// mise environment.
func langLister(a *app) lang.VersionLister {
	return lang.MiseLister{Env: append(os.Environ(), a.env...)}
}

// splitVersions splits comma/space-separated version tokens and trims them.
func splitVersions(args []string) []string {
	joined := strings.Join(args, ",")
	var out []string
	for _, tok := range strings.FieldsFunc(joined, func(r rune) bool { return r == ',' || r == ' ' }) {
		if tok = strings.TrimSpace(tok); tok != "" {
			out = append(out, tok)
		}
	}
	return out
}
