package tools

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/drjzlyan/karya/internal/config"
)

// ProvisionProject installs a project's own pinned runtimes by running
// `mise install` in the project directory with karya's isolated, project-trusting
// environment (env must come from config.Paths.EnvForProject so the project's
// mise.toml/.tool-versions is trusted and layered over karya's global config).
//
// It is best-effort and non-fatal: it provisions the vendored mise first, then
// runs the install, streaming output. `mise install` is idempotent and fast when
// the pinned versions are already present, so only the first open of a project
// incurs real install time. Failures are surfaced as warnings, never aborting the
// session launch.
func ProvisionProject(p config.Paths, projectRoot string, env []string, out, errOut io.Writer) {
	mise, err := EnsureMise(p, out, errOut)
	if err != nil {
		fmt.Fprintf(errOut, "warning: could not provision project runtimes: %v\n", err)
		return
	}
	_ = provisionCmd(mise, projectRoot, env, out, errOut).Run()
}

// provisionCmd builds the `mise install` command for a project. It is separated
// from ProvisionProject so the command shape (working dir + env) is unit-testable
// without running mise.
func provisionCmd(mise, projectRoot string, env []string, out, errOut io.Writer) *exec.Cmd {
	cmd := exec.Command(mise, "install")
	cmd.Dir = projectRoot
	cmd.Env = env
	cmd.Stdout = out
	cmd.Stderr = errOut
	return cmd
}
