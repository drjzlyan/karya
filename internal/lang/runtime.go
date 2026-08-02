package lang

import "io"

// RuntimeManager provisions the language runtimes for a selection into karya's
// isolated prefix. It bundles the two steps that always go together —
// regenerating the isolated mise config from the selection, then running
// `mise install` — behind one reusable seam so both `karya lang`/`install` and
// (future) profile installs share the same runtime provisioning.
//
// It takes explicit path strings rather than a config.Paths so the lang package
// stays decoupled from config. Callers provide an env that pins mise to karya's
// prefix (config.Paths.MiseEnv appended to os.Environ).
type RuntimeManager struct {
	// MiseConfigPath is where the isolated mise config is written
	// (config.Paths.MiseConfig).
	MiseConfigPath string
	// GoPath and CargoHome pin the Go/Rust toolchain state inside karya's prefix
	// (config.Paths.Data/go and .../cargo).
	GoPath    string
	CargoHome string
	// Env pins mise to karya's isolated config/data/cache.
	Env []string
	// Out and ErrOut receive progress and diagnostics.
	Out    io.Writer
	ErrOut io.Writer
}

// Ensure regenerates the isolated mise config from the selection and the
// always-on tools, then installs the runtimes. It is best-effort: ran is false
// (with a nil error) when mise is not yet provisioned, so callers can record the
// selection and continue offline. A non-nil error means the config could not be
// written or `mise install` failed.
func (rm RuntimeManager) Ensure(sel *Selection, always []MiseTool) (ran bool, err error) {
	vars := MiseVars{GoPath: rm.GoPath, CargoHome: rm.CargoHome}
	if err := WriteMiseConfig(rm.MiseConfigPath, sel, vars, always); err != nil {
		return false, err
	}
	return InstallRuntimes(rm.Env, rm.Out, rm.ErrOut)
}
