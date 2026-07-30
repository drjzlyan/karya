package lang

import (
	"fmt"
	"io"
	"os/exec"
)

// InstallRuntimes runs `mise install` against karya's isolated mise config to
// install the selected runtimes, streaming output to out/errOut. env must pin
// mise to the karya prefix (config.Paths.MiseEnv appended to os.Environ). It is
// a no-op returning (false, nil) when mise is not installed, so callers can
// record the selection and continue offline.
func InstallRuntimes(env []string, out, errOut io.Writer) (ran bool, err error) {
	if !MiseInstalled() {
		return false, nil
	}
	cmd := exec.Command("mise", "install")
	cmd.Env = env
	cmd.Stdout = out
	cmd.Stderr = errOut
	if err := cmd.Run(); err != nil {
		return true, fmt.Errorf("mise install: %w", err)
	}
	return true, nil
}
