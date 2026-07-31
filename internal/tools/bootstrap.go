package tools

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/drjzlyan/karya/internal/config"
)

// CoreTool is a runtime dependency karya installs into its own isolated prefix
// via the vendored mise, rather than expecting the user to have provided it.
type CoreTool struct {
	// Name is the label shown in progress output.
	Name string
	// Bin is the command whose presence on PATH means the tool is available.
	Bin string
	// MiseTool is the mise registry key used to install it (e.g. "neovim").
	MiseTool string
	// Essential marks a dependency karya's core cannot run without (tmux, nvim);
	// a failure to provide one aborts the bootstrap with a clear error.
	Essential bool
}

// coreTools are the dependencies needed to launch the IDE itself: the tmux host
// and the Neovim editor. Installing these is enough to bring up a session, so
// the launch path (`karya`, `karya new`) only ensures these for a fast start.
var coreTools = []CoreTool{
	{Name: "tmux", Bin: "tmux", MiseTool: "tmux", Essential: true},
	{Name: "Neovim", Bin: "nvim", MiseTool: "neovim", Essential: true},
}

// toolchainTools are the language toolchain managers karya needs to install
// LSP servers, formatters, and debug adapters: node (npm), go, rust (cargo),
// and uv. They are provisioned by `karya install`, not on every launch, since
// they are only needed when installing per-language tooling.
var toolchainTools = []CoreTool{
	{Name: "Node.js (npm)", Bin: "npm", MiseTool: "node"},
	{Name: "Go toolchain", Bin: "go", MiseTool: "go"},
	{Name: "Rust toolchain", Bin: "cargo", MiseTool: "rust"},
	{Name: "uv (Python)", Bin: "uv", MiseTool: "uv"},
}

// CoreReady reports whether the essential launch dependencies (tmux, Neovim)
// already resolve, so the caller can skip bootstrapping on the common path.
func CoreReady() bool {
	for _, t := range coreTools {
		if t.Essential && !onPath(t.Bin) {
			return false
		}
	}
	return true
}

// EnsureCore makes the essential launch dependencies (tmux, Neovim) available,
// installing any that are missing into karya's isolated prefix via the vendored
// mise. It returns a clear error if an essential tool still cannot be resolved
// afterwards. It is safe to call on every launch: when everything is already
// present it does nothing.
func EnsureCore(p config.Paths, out, errOut io.Writer) error {
	return ensure(p, coreTools, out, errOut)
}

// EnsureToolchains provisions everything EnsureCore does plus the language
// toolchain managers (node, go, rust, uv). It is used by `karya install` so a
// fresh machine can subsequently install the per-language LSP tooling.
func EnsureToolchains(p config.Paths, out, errOut io.Writer) error {
	return ensure(p, append(append([]CoreTool{}, coreTools...), toolchainTools...), out, errOut)
}

// ensure installs any missing tools in specs via the isolated mise. Individual
// non-essential failures are reported but do not abort the run (best-effort,
// matching the LSP installer); a missing essential tool is a hard error.
func ensure(p config.Paths, specs []CoreTool, out, errOut io.Writer) error {
	pending := make([]CoreTool, 0, len(specs))
	for _, t := range specs {
		if onPath(t.Bin) {
			logline(out, "✓ "+t.Name+" already available")
			continue
		}
		pending = append(pending, t)
	}
	if len(pending) == 0 {
		return nil
	}

	mise, err := EnsureMise(p, out, errOut)
	if err != nil {
		return err
	}

	env := miseEnv(p)
	for _, t := range pending {
		logline(out, "installing "+t.Name+" into karya's isolated prefix…")
		if err := runMise(mise, env, out, errOut, "use", "--global", t.MiseTool+"@latest"); err != nil {
			warnline(errOut, fmt.Sprintf("could not install %s: %v", t.Name, err))
		}
	}
	// Regenerate shims so freshly installed tools resolve on karya's PATH.
	_ = runMise(mise, env, io.Discard, io.Discard, "reshim")

	var missing []string
	for _, t := range pending {
		if t.Essential && !onPath(t.Bin) {
			missing = append(missing, t.Name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("could not install %s into karya's prefix — run `karya doctor` for details",
			strings.Join(missing, ", "))
	}
	return nil
}

// miseEnv builds the environment for invoking the isolated mise: the process
// environment plus karya's MISE_* overrides so runtimes, shims, and config all
// stay inside the karya prefix.
func miseEnv(p config.Paths) []string {
	return append(os.Environ(), p.MiseEnv()...)
}

// runMise executes the vendored mise binary with karya's isolated environment.
func runMise(mise string, env []string, out, errOut io.Writer, args ...string) error {
	cmd := exec.Command(mise, args...)
	cmd.Env = env
	cmd.Stdout = out
	cmd.Stderr = errOut
	return cmd.Run()
}

// onPath reports whether bin resolves on the current process PATH. karya
// prepends its managed tool dirs to its own PATH at startup (see cli.newApp),
// so this finds tools installed into the isolated prefix.
func onPath(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

func warnline(errOut io.Writer, msg string) {
	if errOut != nil {
		fmt.Fprintln(errOut, msg)
	}
}
