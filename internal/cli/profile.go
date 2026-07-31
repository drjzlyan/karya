package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/drjzlyan/karya/internal/lang"
	"github.com/drjzlyan/karya/internal/tools"
)

// cmdProfile dispatches `karya profile [list|install <id>]`. Profiles are
// installable bundles of managed tools (core CLI, documentation, and per
// language); installing one provisions everything for that ecosystem into
// karya's isolated prefix.
func cmdProfile(args []string) int {
	a, err := newApp()
	if err != nil {
		return fail(err)
	}
	sub := "list"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "list", "status":
		return profileList(a)
	case "install", "add":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: karya profile install <id>")
			return 2
		}
		return profileInstall(a, args[1])
	default:
		fmt.Fprintln(os.Stderr, "usage: karya profile [list|install <id>]")
		return 2
	}
}

// profileList shows each profile and how many of its tools are installed.
func profileList(a *app) int {
	fmt.Println("Profiles (managed tool bundles):")
	for _, p := range a.reg.Profiles() {
		installed := 0
		for _, id := range p.Tools {
			if _, ok := a.resolver.Resolve(id); ok {
				installed++
			}
		}
		fmt.Printf("  %-12s %2d/%-2d installed\n", p.ID, installed, len(p.Tools))
	}
	fmt.Println("\nInstall one with: karya profile install <id>")
	return 0
}

// profileInstall installs a profile's tools. Language profiles route through the
// language flow so their runtime is provisioned and recorded in the selection;
// the core and docs profiles install their tools directly.
func profileInstall(a *app, id string) int {
	p, ok := a.reg.Profile(id)
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown profile %q (try `karya profile list`)\n", id)
		return 2
	}
	if len(p.Runtimes) > 0 {
		sel, err := loadSelection(a)
		if err != nil {
			return fail(err)
		}
		for _, name := range p.Runtimes {
			if l, ok := lang.Find(name); ok {
				if !sel.Has(name) {
					sel.Set(name, []string{l.Fallback})
				}
			}
		}
		return applySelection(a, sel)
	}

	// Provision the isolated mise so mise-backed tools can install even on a
	// machine that never ran `karya install`.
	if _, err := tools.EnsureMise(a.paths, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}
	results := a.installToolIDs(p.Tools)
	a.refreshToolManifest()
	fmt.Printf("%s (%s): %s\n", p.Name, strings.Join(p.Tools, ", "), tools.Summarize(results))
	return 0
}
