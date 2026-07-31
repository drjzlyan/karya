package cli

import (
	"fmt"
	"os"

	"github.com/drjzlyan/karya/internal/toolreg"
	"github.com/drjzlyan/karya/internal/tools"
)

// cmdTool dispatches `karya tool [list|update <id>|update all]`. It surfaces the
// health of every managed tool and updates tools independently of one another.
func cmdTool(args []string) int {
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
		return toolList(a)
	case "update", "upgrade":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: karya tool update <id>|all")
			return 2
		}
		return toolUpdate(a, args[1:])
	default:
		fmt.Fprintln(os.Stderr, "usage: karya tool [list|update <id>|update all]")
		return 2
	}
}

// categoryOrder is the stable grouping order for health output.
var categoryOrder = []toolreg.Category{
	toolreg.Runtime, toolreg.LanguageServer, toolreg.Formatter,
	toolreg.Linter, toolreg.Debugger, toolreg.BuildTool, toolreg.CLIUtility,
}

// toolList prints the health of every managed tool, grouped by category.
func toolList(a *app) int {
	h := toolreg.NewHealthChecker(a.resolver)
	for _, c := range categoryOrder {
		group := a.reg.ByCategory(c)
		if len(group) == 0 {
			continue
		}
		fmt.Printf("%s:\n", c)
		for _, t := range group {
			s := h.Check(t)
			if s.Installed {
				detail := s.Source.String()
				if s.Version != "" {
					detail += ", " + s.Version
				}
				fmt.Printf("  %-28s ✓ (%s)\n", t.ID, detail)
			} else {
				fmt.Printf("  %-28s ✗ %s\n", t.ID, s.RepairHint)
			}
		}
	}
	return 0
}

// toolUpdate updates one or more tools independently. "all" updates every tool
// whose update strategy supports it; pinned and externally-managed tools are
// reported rather than touched.
func toolUpdate(a *app, ids []string) int {
	// Provision the isolated mise so mise-backed updates can run.
	if _, err := tools.EnsureMise(a.paths, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}
	targets := ids
	if len(ids) == 1 && ids[0] == "all" {
		targets = nil
		for _, t := range a.reg.All() {
			targets = append(targets, t.ID)
		}
	}

	env := append(os.Environ(), a.env...)
	d := tools.NewLayoutDispatcher(a.paths)
	ctx := tools.Context{Env: env, Out: os.Stdout, ErrOut: os.Stderr}
	for _, id := range targets {
		t, ok := a.reg.Get(id)
		if !ok {
			fmt.Fprintf(os.Stderr, "unknown tool %q\n", id)
			continue
		}
		switch t.Update {
		case toolreg.UpdatePinned:
			fmt.Printf("%s is pinned to %s; update karya to change it.\n", id, t.Version)
		case toolreg.UpdateManual:
			fmt.Printf("%s is managed outside karya; nothing to update.\n", id)
		default:
			d.Reinstall(t, ctx)
		}
	}
	a.refreshToolManifest()
	return 0
}
