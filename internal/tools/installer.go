package tools

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/drjzlyan/karya/internal/config"
	"github.com/drjzlyan/karya/internal/toolreg"
)

// Context carries the isolated environment and destination directories an
// installer needs. It is built from an Installer (or, in later phases, from
// config.Paths per a tool's Location) and passed to every Method. Nothing is
// written outside these directories and the user's global environment is never
// mutated.
type Context struct {
	// ToolsDir is karya's tool prefix root (config.Paths.ToolsDir).
	ToolsDir string
	// BinDir is where tool binaries land and are looked up (ToolsDir/bin).
	BinDir string
	// Env is the environment for child installers (os.Environ plus karya's
	// isolated overrides). Methods add their own tool-specific vars on top.
	Env []string
	// Out and ErrOut receive progress and diagnostics.
	Out    io.Writer
	ErrOut io.Writer
}

// env returns the context environment, defaulting to the process environment.
func (c Context) env() []string {
	if c.Env != nil {
		return c.Env
	}
	return os.Environ()
}

// run executes name with the isolated env plus extraEnv, wiring stdout/stderr to
// the context's writers.
func (c Context) run(extraEnv []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Env = append(append([]string{}, c.env()...), extraEnv...)
	cmd.Stdout = c.Out
	cmd.Stderr = c.ErrOut
	return cmd.Run()
}

// available reports whether a tool already resolves: its Artifact exists under
// ToolsDir, or its Executable is in BinDir or on the ambient PATH.
func (c Context) available(t toolreg.Tool) bool {
	if t.Artifact != "" {
		if _, err := os.Stat(filepath.Join(c.ToolsDir, t.Artifact)); err == nil {
			return true
		}
	}
	if t.Executable == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(c.BinDir, t.Executable)); err == nil {
		return true
	}
	_, err := exec.LookPath(t.Executable)
	return err == nil
}

// Method installs one class of tool into karya's isolated prefix. Each install
// method (mise, uv, npm, go, rustup, jdtls, lombok, vsix, detect) is one Method,
// so adding a new provisioning path means adding an implementation rather than
// editing a growing switch.
type Method interface {
	// Available reports whether the tool already resolves (detect-first).
	Available(t toolreg.Tool, ctx Context) bool
	// Install provisions the tool into the context's directories.
	Install(t toolreg.Tool, ctx Context) error
	// CurrentVersion returns the installed version, or "" if unknown. Populated
	// by the version manager phase; the base default returns "".
	CurrentVersion(t toolreg.Tool, ctx Context) string
	// LatestVersion returns the newest available version, or "" if unknown or
	// unsupported. Populated by the version manager phase.
	LatestVersion(t toolreg.Tool, ctx Context) string
}

// base provides default Method behavior — detect-first availability and unknown
// versions — so a concrete method only implements Install (plus version probes
// where it can).
type base struct{}

func (base) Available(t toolreg.Tool, ctx Context) bool  { return ctx.available(t) }
func (base) CurrentVersion(toolreg.Tool, Context) string { return "" }
func (base) LatestVersion(toolreg.Tool, Context) string  { return "" }

// Dispatcher routes each tool to the Method for its InstallMethod. It is the
// side-effect counterpart to toolreg.Registry: the registry says what to install
// and in what order; the Dispatcher does it, detect-first and best-effort.
type Dispatcher struct {
	methods map[toolreg.InstallMethod]Method
	// layout, when set, maps a tool to its per-category install dirs so tools
	// land under tools/<category> instead of one shared bin. When nil the
	// Context's ToolsDir/BinDir are used unchanged (legacy single-prefix mode).
	layout func(toolreg.Tool) (toolsDir, binDir string)
}

// NewDispatcher wires the built-in installers in legacy single-prefix mode: all
// tools install into the Context's ToolsDir/BinDir.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{methods: map[toolreg.InstallMethod]Method{
		toolreg.MethodMise:   miseMethod{},
		toolreg.MethodUV:     uvMethod{},
		toolreg.MethodNPM:    npmMethod{},
		toolreg.MethodGo:     goMethod{},
		toolreg.MethodRustup: rustupMethod{},
		toolreg.MethodJDTLS:  jdtlsMethod{},
		toolreg.MethodLombok: lombokMethod{},
		toolreg.MethodVSIX:   vsixMethod{},
		toolreg.MethodDetect: detectMethod{},
	}}
}

// NewLayoutDispatcher wires the built-in installers and routes each tool into its
// category directory (tools/core, tools/docs, tools/<lang>) based on the tool's
// Location. Tools without a category (runtimes, unset) fall back to the shared
// tool prefix. mise-installed tools resolve via shims regardless, so their
// category dir is immaterial.
func NewLayoutDispatcher(p config.Paths) *Dispatcher {
	d := NewDispatcher()
	d.layout = func(t toolreg.Tool) (string, string) {
		name := categoryName(t.Location)
		if name == "" {
			return p.ToolsDir(), p.ToolsBin()
		}
		return p.ToolCategoryDir(name), p.ToolCategoryBin(name)
	}
	return d
}

// categoryName maps a tool's install location to its on-disk category directory
// name, or "" for the shared prefix (runtimes and unset locations).
func categoryName(loc toolreg.InstallLocation) string {
	switch loc.Kind {
	case toolreg.LocCore:
		return "core"
	case toolreg.LocDocs:
		return "docs"
	case toolreg.LocLang:
		return loc.Lang
	default:
		return ""
	}
}

// Install processes each tool and returns per-tool results. It never returns an
// error itself; individual failures are captured in Result.Err so one broken
// tool does not abort the rest.
func (d *Dispatcher) Install(tools []toolreg.Tool, ctx Context) []Result {
	results := make([]Result, 0, len(tools))
	for _, t := range tools {
		results = append(results, d.one(t, ctx))
	}
	return results
}

func (d *Dispatcher) one(t toolreg.Tool, ctx Context) Result {
	m := d.methods[t.Method]
	if m == nil {
		return Result{Tool: t.Name, Status: Failed, Err: fmt.Errorf("unknown install method %q", t.Method)}
	}
	// Route the tool into its category dir when a layout is configured.
	if d.layout != nil {
		ctx.ToolsDir, ctx.BinDir = d.layout(t)
	}
	if m.Available(t, ctx) {
		logf(ctx.Out, "✓ %s already available", t.Name)
		return Result{Tool: t.Name, Status: Skipped}
	}
	// Detect-only tools ship through channels karya must not mutate; a missing one
	// is reported (with its hint) rather than installed.
	if t.Method == toolreg.MethodDetect {
		warnf(ctx.ErrOut, "%s not found — %s", t.Name, t.Hint)
		return Result{Tool: t.Name, Status: Missing}
	}
	logf(ctx.Out, "installing %s…", t.Name)
	// Ensure the destination dirs exist so npm/uv/go installers have somewhere to
	// write (category dirs are created on demand, not up front).
	if ctx.BinDir != "" {
		_ = os.MkdirAll(ctx.BinDir, 0o755)
	}
	if ctx.ToolsDir != "" {
		_ = os.MkdirAll(ctx.ToolsDir, 0o755)
	}
	if err := m.Install(t, ctx); err != nil {
		warnf(ctx.ErrOut, "could not install %s: %v", t.Name, err)
		return Result{Tool: t.Name, Status: Failed, Err: err}
	}
	logf(ctx.Out, "✓ installed %s", t.Name)
	return Result{Tool: t.Name, Status: Installed}
}

func logf(out io.Writer, format string, a ...any) {
	if out != nil {
		fmt.Fprintf(out, "[tools] "+format+"\n", a...)
	}
}

func warnf(errOut io.Writer, format string, a ...any) {
	if errOut != nil {
		fmt.Fprintf(errOut, "[tools] "+format+"\n", a...)
	}
}
