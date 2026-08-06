package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/drjzlyan/karya/internal/config"
	"github.com/drjzlyan/karya/internal/skills"
)

// defaultRegistry is karya's built-in skills registry (DESIGN.md §9). Users add
// more with `karya skills registry add <url>`.
const defaultRegistry = "https://raw.githubusercontent.com/karya-dev/skills/main"

// cmdSkills implements `karya skills search|install|remove|list|registry`.
func cmdSkills(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: karya skills list|search|install|remove|registry …")
		return 2
	}
	a, err := newApp()
	if err != nil {
		return fail(err)
	}
	store := skills.Store{Root: a.paths.SkillsDir()}
	sub, rest := args[0], args[1:]
	switch sub {
	case "list":
		return skillsList(store)
	case "search":
		return skillsSearch(a.paths, store, strings.Join(rest, " "))
	case "install":
		return skillsInstall(a.paths, store, rest)
	case "remove":
		return skillsRemove(store, rest)
	case "registry":
		return skillsRegistry(a.paths, rest)
	default:
		fmt.Fprintf(os.Stderr, "unknown skills subcommand %q\n", sub)
		return 2
	}
}

func skillsList(store skills.Store) int {
	names := store.List()
	if len(names) == 0 {
		fmt.Println("No skills installed. Find some with `karya skills search`.")
		return 0
	}
	fmt.Println("Installed skills:")
	for _, n := range names {
		fmt.Printf("  %s\n", n)
	}
	return 0
}

func skillsSearch(p config.Paths, store skills.Store, query string) int {
	seen := map[string]bool{}
	var found int
	for _, reg := range registries(p) {
		idx, err := skills.LoadIndex(sourceFor(reg))
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: registry %s: %v\n", reg, err)
			continue
		}
		for _, e := range idx.Search(query) {
			if seen[e.Name] {
				continue
			}
			seen[e.Name] = true
			mark := ""
			if store.Installed(e.Name) {
				mark = "  [installed]"
			}
			fmt.Printf("  %-24s %-8s %s%s\n", e.Name, e.Version, e.Description, mark)
			found++
		}
	}
	if found == 0 {
		fmt.Println("No matching skills.")
	}
	return 0
}

func skillsInstall(p config.Paths, store skills.Store, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: karya skills install <name>")
		return 2
	}
	name := args[0]
	for _, reg := range registries(p) {
		src := sourceFor(reg)
		idx, err := skills.LoadIndex(src)
		if err != nil {
			continue
		}
		if entry, ok := idx.Find(name); ok {
			if err := store.Install(entry, src); err != nil {
				return fail(err)
			}
			fmt.Printf("installed %s %s from %s\n", entry.Name, entry.Version, reg)
			return 0
		}
	}
	fmt.Fprintf(os.Stderr, "skill %q not found in any registry\n", name)
	return 1
}

func skillsRemove(store skills.Store, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: karya skills remove <name>")
		return 2
	}
	if !store.Installed(args[0]) {
		fmt.Fprintf(os.Stderr, "skill %q is not installed\n", args[0])
		return 1
	}
	if err := store.Remove(args[0]); err != nil {
		return fail(err)
	}
	fmt.Printf("removed %s\n", args[0])
	return 0
}

func skillsRegistry(p config.Paths, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: karya skills registry add <url> | list")
		return 2
	}
	switch args[0] {
	case "list":
		for _, r := range registries(p) {
			fmt.Println("  " + r)
		}
		return 0
	case "add":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: karya skills registry add <url>")
			return 2
		}
		if err := addRegistry(p, args[1]); err != nil {
			return fail(err)
		}
		fmt.Println("added registry:", args[1])
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown registry subcommand %q\n", args[0])
		return 2
	}
}

// registries returns the default registry plus any the user added.
func registries(p config.Paths) []string {
	regs := []string{defaultRegistry}
	if data, err := os.ReadFile(p.RegistriesFile()); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if l := strings.TrimSpace(line); l != "" {
				regs = append(regs, l)
			}
		}
	}
	return regs
}

// addRegistry appends a registry URL to the user's registries file.
func addRegistry(p config.Paths, url string) error {
	if err := os.MkdirAll(filepath.Dir(p.RegistriesFile()), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(p.RegistriesFile(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintln(f, strings.TrimSpace(url))
	return err
}

// sourceFor builds a Source from a registry reference: an http(s) URL uses an
// HTTP source; anything else is treated as a local directory (a cloned
// registry, or tests).
func sourceFor(ref string) skills.Source {
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return skills.NewHTTPSource(ref)
	}
	return skills.DirSource{Root: ref}
}
