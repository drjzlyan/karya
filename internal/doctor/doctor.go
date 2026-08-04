// Package doctor runs karya's health checks: required tools and their versions,
// the isolation invariants, whether the embedded config has been extracted, which
// coding agents are available, and the per-language editor tooling for the
// current language selection.
//
// The check logic is pure and driven by an injected Probe, so the whole report
// is exercised by hermetic unit tests; the CLI supplies a Probe backed by the
// real system (exec.LookPath, the tools installer, agent detection, …). Defaults
// fill any nil Probe field, so callers only set what they want to override.
package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/drjzlyan/karya/internal/agent"
	"github.com/drjzlyan/karya/internal/config"
	"github.com/drjzlyan/karya/internal/lang"
	"github.com/drjzlyan/karya/internal/toolreg"
)

// Level is the severity of a single check.
type Level int

const (
	// OK — the check passed.
	OK Level = iota
	// Warn — a non-fatal gap: karya works, but something optional is missing.
	Warn
	// Problem — a required piece is missing; core functionality will not work.
	Problem
)

// Check is one health-check result.
type Check struct {
	Group  string
	Name   string
	Level  Level
	Detail string
}

// Report is the full set of checks from a doctor run.
type Report struct {
	Checks []Check
}

func (r *Report) add(group, name string, level Level, detail string) {
	r.Checks = append(r.Checks, Check{Group: group, Name: name, Level: level, Detail: detail})
}

// Count returns how many checks are at the given level.
func (r Report) Count(level Level) int {
	n := 0
	for _, c := range r.Checks {
		if c.Level == level {
			n++
		}
	}
	return n
}

// Healthy reports whether the run found no Problem-level checks.
func (r Report) Healthy() bool { return r.Count(Problem) == 0 }

// Probe supplies the environment doctor inspects. Any nil function is filled by
// withDefaults with a real, system-backed implementation.
type Probe struct {
	Paths        config.Paths
	Selection    *lang.Selection
	KaryaVersion string

	// LookPath resolves a command name to its path and whether it exists.
	LookPath func(string) (string, bool)
	// Version returns a best-effort version string for a command, or "".
	Version func(bin string) string
	// Exists reports whether a filesystem path exists.
	Exists func(path string) bool
	// Detected returns the coding agents available on the machine.
	Detected func() []string
	// Registry is the tool registry doctor reports against.
	Registry *toolreg.Registry
	// ToolInstalled reports whether a tool (by registry ID) resolves in karya's
	// prefix or on PATH.
	ToolInstalled func(id string) bool

	// CheckUpdates enables the (network) managed-tool update check.
	CheckUpdates bool
	// Updates returns the managed tools that have an update available. Called only
	// when CheckUpdates is set; defaulted to a mise-backed VersionManager.
	Updates func() []toolreg.VersionInfo
}

func (p Probe) withDefaults() Probe {
	if p.LookPath == nil {
		p.LookPath = func(name string) (string, bool) {
			path, err := exec.LookPath(name)
			return path, err == nil
		}
	}
	if p.Version == nil {
		p.Version = defaultVersion
	}
	if p.Exists == nil {
		p.Exists = func(path string) bool { _, err := os.Stat(path); return err == nil }
	}
	if p.Detected == nil {
		p.Detected = agent.Detect
	}
	if p.Registry == nil {
		p.Registry = toolreg.New()
	}
	if p.ToolInstalled == nil {
		rv := toolreg.NewResolver(p.Paths, p.Registry)
		p.ToolInstalled = func(id string) bool { _, ok := rv.Resolve(id); return ok }
	}
	if p.Updates == nil {
		reg := p.Registry
		p.Updates = func() []toolreg.VersionInfo {
			vm := toolreg.NewVersionManager(reg, append(os.Environ(), p.Paths.MiseEnv()...))
			ids := make([]string, 0)
			for _, t := range reg.All() {
				ids = append(ids, t.ID)
			}
			return vm.Updates(ids)
		}
	}
	return p
}

// coreTool is a command karya orchestrates. Essential tools are Problems when
// missing (karya's core does not work without them); the rest are Warnings.
type coreTool struct {
	name      string
	bin       string
	essential bool
	hint      string
}

var coreTools = []coreTool{
	{"tmux", "tmux", true, "run `karya install` to set up karya's isolated runtime"},
	{"Neovim", "nvim", true, "run `karya install` to set up karya's isolated runtime"},
	{"git", "git", false, "install git for version control"},
	{"mise", "mise", false, "run `karya install` to provision karya's isolated mise"},
	{"lazygit", "lazygit", false, "run `karya install` (managed) — powers the in-session git UI (Ctrl-a g)"},
	{"starship", "starship", false, "run `karya install` (managed) — renders the pane prompt"},
}

// Run gathers every check into a Report.
func Run(p Probe) Report {
	p = p.withDefaults()
	var r Report

	r.add("karya", "version", OK, p.KaryaVersion)
	checkIsolation(&r, p)
	checkConfigExtracted(&r, p)
	checkCoreTools(&r, p)
	checkAgents(&r, p)
	checkLanguages(&r, p)
	if p.CheckUpdates {
		checkUpdates(&r, p)
	}

	return r
}

// checkUpdates reports managed tools with a newer version available. It runs only
// with `karya doctor --check-updates` since it queries mise over the network.
func checkUpdates(r *Report, p Probe) {
	ups := p.Updates()
	if len(ups) == 0 {
		r.add("updates", "managed tools", OK, "all up to date")
		return
	}
	for _, vi := range ups {
		r.add("updates", vi.ID, Warn, "update available: "+vi.Installed+" → "+vi.Latest)
	}
	r.add("updates", "summary", Warn,
		fmt.Sprintf("%d update(s) available — run `karya tool update all`", len(ups)))
}

// checkIsolation confirms every karya directory is namespaced under the karya
// prefix — the guarantee that karya never touches the user's own config.
func checkIsolation(r *Report, p Probe) {
	roots := map[string]string{
		"Config": p.Paths.Config, "Data": p.Paths.Data,
		"State": p.Paths.State, "Cache": p.Paths.Cache,
	}
	for name, dir := range roots {
		if filepath.Base(dir) != config.AppName {
			r.add("karya", "isolation", Problem,
				name+" dir is not namespaced under "+config.AppName+": "+dir)
			return
		}
	}
	r.add("karya", "isolation", OK, "all state under a karya-owned prefix ("+p.Paths.Config+")")
}

// checkConfigExtracted reports whether `karya install` has extracted the embedded
// editor and tmux config into karya's prefix.
func checkConfigExtracted(r *Report, p Probe) {
	if p.Exists(p.Paths.NvimConfig()) {
		r.add("karya", "editor config", OK, "extracted at "+p.Paths.NvimConfig())
	} else {
		r.add("karya", "editor config", Warn, "not extracted yet — run `karya install`")
	}
	if p.Exists(p.Paths.TmuxConf()) {
		r.add("karya", "tmux config", OK, "extracted at "+p.Paths.TmuxConf())
	} else {
		r.add("karya", "tmux config", Warn, "not extracted yet — run `karya install`")
	}
}

// checkCoreTools verifies each orchestrated command is present, with its version.
func checkCoreTools(r *Report, p Probe) {
	for _, t := range coreTools {
		if _, ok := p.LookPath(t.bin); ok {
			r.add("tools", t.name, OK, detailWithVersion("found", p.Version(t.bin)))
			continue
		}
		level := Warn
		if t.essential {
			level = Problem
		}
		r.add("tools", t.name, level, "not found — "+t.hint)
	}
}

// checkAgents reports the detected coding agents.
func checkAgents(r *Report, p Probe) {
	detected := p.Detected()
	if len(detected) == 0 {
		r.add("agents", "coding agent", Warn,
			"none detected — install any of: "+strings.Join(agent.Known, ", ")+
				" (or use `karya dev -a none`)")
		return
	}
	r.add("agents", "coding agent", OK, "detected: "+strings.Join(detected, ", "))
}

// checkLanguages reports the current language selection and whether each
// selected language's editor tooling (plus the always-on servers) is installed.
func checkLanguages(r *Report, p Probe) {
	var langs []string
	if p.Selection != nil {
		langs = p.Selection.Langs()
	}
	if len(langs) == 0 {
		r.add("languages", "selection", Warn, "no languages selected — run `karya lang`")
	} else {
		labels := make([]string, 0, len(langs))
		for _, name := range langs {
			labels = append(labels, name+" "+strings.Join(p.Selection.Versions(name), ","))
		}
		r.add("languages", "selection", OK, "selected: "+strings.Join(labels, "; "))
	}

	ids := toolreg.AlwaysOnIDs()
	for _, name := range langs {
		ids = append(ids, p.Registry.LanguageIDs(name)...)
	}
	for _, id := range ids {
		t, ok := p.Registry.Get(id)
		if !ok {
			continue
		}
		if p.ToolInstalled(id) {
			r.add("languages", t.Name, OK, "installed")
			continue
		}
		detail := "missing — run `karya install`"
		if t.Hint != "" {
			detail = "missing — " + t.Hint
		}
		r.add("languages", t.Name, Warn, detail)
	}
}

// detailWithVersion appends a version suffix to a detail when one is known.
func detailWithVersion(base, version string) string {
	if version == "" {
		return base
	}
	return base + " (" + version + ")"
}

// defaultVersion runs `<bin> --version` and returns its trimmed first line, or ""
// when the command is unavailable or does not support the flag.
func defaultVersion(bin string) string {
	out, err := exec.Command(bin, "--version").Output() //nolint:gosec // fixed known tool names
	if err != nil || len(out) == 0 {
		return ""
	}
	line, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	return strings.TrimSpace(line)
}
