package tools

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Installer installs tools into karya's isolated prefix. All installs are
// best-effort and detect-first: a tool already resolvable on PATH (or already
// present under ToolsDir) is skipped. Nothing is written outside ToolsDir, and
// the user's Homebrew/global env is never mutated.
type Installer struct {
	// ToolsDir is karya's tool prefix root (config.Paths.ToolsDir).
	ToolsDir string
	// BinDir is where tool binaries land and are looked up (ToolsDir/bin).
	BinDir string
	// Env is the environment for child installers (os.Environ plus karya's
	// isolated overrides). Installers add their own tool-specific vars on top.
	Env []string
	// Out and ErrOut receive progress and diagnostics.
	Out    io.Writer
	ErrOut io.Writer
}

// Status is the outcome of attempting one tool.
type Status int

const (
	// Installed means karya installed the tool this run.
	Installed Status = iota
	// Skipped means the tool was already available.
	Skipped
	// Missing means a KindDetect tool is absent (a hint was printed).
	Missing
	// Failed means installation was attempted and errored.
	Failed
)

// Result pairs a tool with its outcome.
type Result struct {
	Tool   string
	Status Status
	Err    error
}

// Install processes each spec and returns per-tool results. It never returns an
// error itself; individual failures are captured in Result.Err so one broken
// tool does not abort the rest (matching the shell installer's behavior).
func (in Installer) Install(specs []ToolSpec) []Result {
	results := make([]Result, 0, len(specs))
	for _, s := range specs {
		results = append(results, in.one(s))
	}
	return results
}

func (in Installer) one(s ToolSpec) Result {
	if in.available(s) {
		in.logf("✓ %s already available", s.Name)
		return Result{Tool: s.Name, Status: Skipped}
	}
	in.logf("installing %s…", s.Name)
	var err error
	switch s.Kind {
	case KindUV:
		err = in.installUV(s)
	case KindNPM:
		err = in.installNPM(s)
	case KindGo:
		err = in.installGo(s)
	case KindRustup:
		err = in.installRustup(s)
	case KindDetect:
		in.warnf("%s not found — %s", s.Name, s.Hint)
		return Result{Tool: s.Name, Status: Missing}
	case KindJDTLS:
		err = in.installJDTLS(s)
	case KindLombok:
		err = in.installLombok(s)
	case KindVSIX:
		err = in.installVSIX(s)
	default:
		err = fmt.Errorf("unknown install kind %q", s.Kind)
	}
	if err != nil {
		in.warnf("could not install %s: %v", s.Name, err)
		return Result{Tool: s.Name, Status: Failed, Err: err}
	}
	in.logf("✓ installed %s", s.Name)
	return Result{Tool: s.Name, Status: Installed}
}

// available reports whether a tool is already usable: its Bin resolves in
// karya's BinDir or on the ambient PATH, or its Artifact exists under ToolsDir.
func (in Installer) available(s ToolSpec) bool {
	if s.Artifact != "" {
		if _, err := os.Stat(filepath.Join(in.ToolsDir, s.Artifact)); err == nil {
			return true
		}
	}
	if s.Bin == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(in.BinDir, s.Bin)); err == nil {
		return true
	}
	_, err := exec.LookPath(s.Bin)
	return err == nil
}

// ── Installers ──────────────────────────────────────────────────────────────

// installUV installs a Python tool with uv, redirecting its bin output to
// karya's BinDir (uv defaults to ~/.local/bin, which may not be karya's).
func (in Installer) installUV(s ToolSpec) error {
	if _, err := exec.LookPath("uv"); err != nil {
		return fmt.Errorf("uv not found on PATH (required for Python tools)")
	}
	pkg := s.Pkg
	if s.Version != "" && s.Version != "latest" {
		pkg += "==" + s.Version
	}
	extra := []string{"UV_TOOL_BIN_DIR=" + in.BinDir}
	return in.run(extra, "uv", "tool", "install", "--force", pkg)
}

// installNPM installs a Node package into an isolated npm prefix (ToolsDir), so
// binaries land in ToolsDir/bin and nothing touches a global npm root.
func (in Installer) installNPM(s ToolSpec) error {
	if _, err := exec.LookPath("npm"); err != nil {
		return fmt.Errorf("npm not found on PATH")
	}
	pkg := s.Pkg
	if s.Version != "" && s.Version != "latest" {
		pkg += "@" + s.Version
	}
	return in.run(nil, "npm", "install", "-g", "--prefix", in.ToolsDir, pkg)
}

// installGo installs a Go tool with GOBIN pinned to karya's BinDir.
func (in Installer) installGo(s ToolSpec) error {
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("go not found on PATH")
	}
	return in.run([]string{"GOBIN=" + in.BinDir}, "go", "install", s.Pkg)
}

// installRustup adds a rustup component. This is the one tool class karya cannot
// fully isolate: components attach to the active rustup toolchain. It is
// detect-first (skipped when already present) and best-effort.
func (in Installer) installRustup(s ToolSpec) error {
	if _, err := exec.LookPath("rustup"); err != nil {
		return fmt.Errorf("rustup not found on PATH (install rustup for Rust tooling)")
	}
	return in.run(nil, "rustup", "component", "add", s.Pkg)
}

// run executes a command with karya's isolated env plus any extra vars, wiring
// stdout/stderr to the installer's writers.
func (in Installer) run(extraEnv []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Env = append(append([]string{}, in.env()...), extraEnv...)
	cmd.Stdout = in.Out
	cmd.Stderr = in.ErrOut
	return cmd.Run()
}

// env returns the installer environment, defaulting to the process environment
// when Env is unset.
func (in Installer) env() []string {
	if in.Env != nil {
		return in.Env
	}
	return os.Environ()
}

func (in Installer) logf(format string, a ...any) {
	if in.Out != nil {
		fmt.Fprintf(in.Out, "[tools] "+format+"\n", a...)
	}
}

func (in Installer) warnf(format string, a ...any) {
	if in.ErrOut != nil {
		fmt.Fprintf(in.ErrOut, "[tools] "+format+"\n", a...)
	}
}

// Summarize renders a one-line count of results for the CLI.
func Summarize(results []Result) string {
	var installed, skipped, missing, failed int
	for _, r := range results {
		switch r.Status {
		case Installed:
			installed++
		case Skipped:
			skipped++
		case Missing:
			missing++
		case Failed:
			failed++
		}
	}
	parts := []string{
		fmt.Sprintf("%d installed", installed),
		fmt.Sprintf("%d already present", skipped),
	}
	if missing > 0 {
		parts = append(parts, fmt.Sprintf("%d need manual install", missing))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}
	return strings.Join(parts, ", ")
}
