package cli

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/drjzlyan/karya/internal/prefs"
	"github.com/drjzlyan/karya/internal/toolreg"
	"github.com/drjzlyan/karya/internal/tools"
)

// cmdTool dispatches `karya tool [list [--check-updates]|update <id>|update all]`.
// It surfaces the health of every managed tool and updates tools independently.
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
		fs := flag.NewFlagSet("tool list", flag.ContinueOnError)
		check := fs.Bool("check-updates", false, "query mise for available updates (network)")
		if err := fs.Parse(argsAfter(args)); err != nil {
			return 2
		}
		return toolList(a, *check)
	case "update", "upgrade":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: karya tool update <id>|all")
			return 2
		}
		return toolUpdate(a, args[1:])
	default:
		fmt.Fprintln(os.Stderr, "usage: karya tool [list [--check-updates]|update <id>|update all]")
		return 2
	}
}

// argsAfter returns the args following the subcommand (for flag parsing).
func argsAfter(args []string) []string {
	if len(args) <= 1 {
		return nil
	}
	return args[1:]
}

// categoryOrder is the stable grouping order for health output.
var categoryOrder = []toolreg.Category{
	toolreg.Runtime, toolreg.LanguageServer, toolreg.Formatter,
	toolreg.Linter, toolreg.Debugger, toolreg.BuildTool, toolreg.CLIUtility,
}

// toolList prints the health of every managed tool, grouped by category. With
// checkUpdates it also queries mise for available updates, annotates each tool,
// and records the check time per tool in tools.state.
func toolList(a *app, checkUpdates bool) int {
	h := toolreg.NewHealthChecker(a.resolver)
	updates := map[string]toolreg.VersionInfo{}
	var state *prefs.Store
	if checkUpdates {
		vm := toolreg.NewVersionManager(a.reg, append(os.Environ(), a.env...))
		ids := make([]string, 0)
		for _, t := range a.reg.All() {
			ids = append(ids, t.ID)
		}
		state = prefs.New(a.paths.ToolsStateFile())
		now := time.Now().UTC().Format(time.RFC3339)
		for _, vi := range vm.Query(ids) {
			updates[vi.ID] = vi
			_ = state.Set("checked."+vi.ID, now)
		}
	}

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
				line := fmt.Sprintf("  %-28s ✓ (%s)", t.ID, detail)
				if vi, ok := updates[t.ID]; ok && vi.UpdateAvailable {
					line += fmt.Sprintf("  update: %s → %s", vi.Installed, vi.Latest)
				}
				fmt.Println(line)
			} else {
				fmt.Printf("  %-28s ✗ %s\n", t.ID, s.RepairHint)
			}
		}
	}
	if checkUpdates {
		fmt.Printf("\nUpdate check recorded in %s. Run `karya tool update <id>|all` to apply.\n",
			a.paths.ToolsStateFile())
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
