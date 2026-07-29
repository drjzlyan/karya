package cli

import (
	"os"

	"github.com/drjzlyan/karya/internal/assets"
	"github.com/drjzlyan/karya/internal/config"
	"github.com/drjzlyan/karya/internal/tmuxx"
)

// app bundles the resolved environment shared by commands: karya paths, the
// isolated child-process env, and a tmux client bound to karya's server.
type app struct {
	paths config.Paths
	bin   string // absolute path to the running karya binary
	env   []string
	tmux  *tmuxx.Tmux
}

// newApp resolves paths, ensures karya-owned dirs exist, extracts the tmux
// config, and returns a ready tmux client. It keeps all isolation concerns in
// one place so every command inherits the same guarantees.
func newApp() (*app, error) {
	p := config.Resolve()
	if err := p.EnsureDirs(); err != nil {
		return nil, err
	}
	bin, err := os.Executable()
	if err != nil || bin == "" {
		bin = "karya"
	}
	// Keep the embedded tmux config in sync on every run (cheap, idempotent).
	if err := assets.ExtractTmuxConf(p.TmuxConf(), bin); err != nil {
		return nil, err
	}
	env := p.Env(bin)
	return &app{
		paths: p,
		bin:   bin,
		env:   env,
		tmux:  tmuxx.New(config.TmuxSocket, p.TmuxConf(), env),
	}, nil
}
