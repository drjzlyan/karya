package tools

import (
	"fmt"
	"io"
	"strings"

	"github.com/drjzlyan/karya/internal/toolreg"
)

// Installer installs tools into karya's isolated prefix. It is a thin facade
// over Dispatcher, retained for callers that still describe tools as ToolSpec
// (the CLI, doctor). All installs are best-effort and detect-first: a tool
// already resolvable on PATH (or present under ToolsDir) is skipped, nothing is
// written outside ToolsDir, and the user's global environment is never mutated.
type Installer struct {
	// ToolsDir is karya's tool prefix root (config.Paths.ToolsDir).
	ToolsDir string
	// BinDir is where tool binaries land and are looked up (ToolsDir/bin).
	BinDir string
	// Env is the environment for child installers (os.Environ plus karya's
	// isolated overrides).
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
	// Missing means a detect-only tool is absent (a hint was printed).
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

// context builds the Dispatcher Context from the Installer's fields. The structs
// share a field layout, so this is a direct conversion.
func (in Installer) context() Context { return Context(in) }

// Install processes each spec and returns per-tool results by delegating to the
// Dispatcher. It never returns an error itself; individual failures are captured
// in Result.Err so one broken tool does not abort the rest.
func (in Installer) Install(specs []ToolSpec) []Result {
	tools := make([]toolreg.Tool, len(specs))
	for i, s := range specs {
		tools[i] = specToTool(s)
	}
	return NewDispatcher().Install(tools, in.context())
}

// Available reports whether a tool is installed and usable in karya's prefix. It
// is the read-only probe `karya doctor` uses to check per-language tooling.
func (in Installer) Available(s ToolSpec) bool { return in.context().available(specToTool(s)) }

// available and one are retained for the package's own tests, delegating to the
// Context/Dispatcher so the detect-first behavior lives in one place.
func (in Installer) available(s ToolSpec) bool { return in.context().available(specToTool(s)) }
func (in Installer) one(s ToolSpec) Result     { return NewDispatcher().one(specToTool(s), in.context()) }

// specToTool bridges the legacy ToolSpec catalog onto the toolreg.Tool the
// Dispatcher installs. It is a translation shim used while callers still build
// ToolSpecs; it is removed once the catalog is fully driven by toolreg.
func specToTool(s ToolSpec) toolreg.Tool {
	return toolreg.Tool{
		ID:         s.Name,
		Name:       s.Name,
		Method:     kindToMethod(s.Kind),
		Executable: s.Bin,
		Artifact:   s.Artifact,
		Pkg:        s.Pkg,
		Version:    s.Version,
		Hint:       s.Hint,
	}
}

// kindToMethod maps a legacy install Kind to its toolreg.InstallMethod.
func kindToMethod(k Kind) toolreg.InstallMethod {
	switch k {
	case KindUV:
		return toolreg.MethodUV
	case KindNPM:
		return toolreg.MethodNPM
	case KindGo:
		return toolreg.MethodGo
	case KindRustup:
		return toolreg.MethodRustup
	case KindMise:
		return toolreg.MethodMise
	case KindDetect:
		return toolreg.MethodDetect
	case KindJDTLS:
		return toolreg.MethodJDTLS
	case KindLombok:
		return toolreg.MethodLombok
	case KindVSIX:
		return toolreg.MethodVSIX
	default:
		return toolreg.InstallMethod(k)
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
