package cli

import (
	"os"

	"github.com/drjzlyan/karya/internal/assets"
	"github.com/drjzlyan/karya/internal/config"
	"github.com/drjzlyan/karya/internal/prefs"
	"github.com/drjzlyan/karya/internal/tmuxx"
)

// app bundles the resolved environment shared by commands: karya paths, the
// isolated child-process env, a tmux client bound to karya's server, and the
// per-project preference store.
type app struct {
	paths config.Paths
	bin   string // absolute path to the running karya binary
	env   []string
	tmux  *tmuxx.Tmux
	prefs *prefs.Store
}

// newApp resolves paths, ensures karya-owned dirs exist, extracts the tmux
// config, and returns a ready tmux client. It keeps all isolation concerns in
// one place so every command inherits the same guarantees.
func newApp() (*app, error) {
	p := config.Resolve()
	if err := p.EnsureDirs(); err != nil {
		return nil, err
	}
	// Let karya resolve and run the tools it installed into its isolated prefix
	// (tmux, Neovim, agents, toolchain managers) without relying on the user's
	// PATH or global mise.
	p.ActivateManagedEnv()
	bin, err := os.Executable()
	if err != nil || bin == "" {
		bin = "karya"
	}
	// Keep the embedded tmux config in sync on every run (cheap, idempotent).
	if err := assets.ExtractTmuxConf(p.TmuxConf(), bin); err != nil {
		return nil, err
	}
	// Extract the embedded Neovim config when it is missing or the binary shipped
	// a newer version. Cheap (a content hash + manifest compare) on the common
	// path where nothing changed; plugins bootstrap lazily on first editor launch.
	if _, err := assets.EnsureNvimConfig(p.NvimConfig()); err != nil {
		return nil, err
	}
	env := p.Env(bin)
	return &app{
		paths: p,
		bin:   bin,
		env:   env,
		tmux:  tmuxx.New(config.TmuxSocket, p.TmuxConf(), env),
		prefs: prefs.New(p.PrefsFile()),
	}, nil
}
