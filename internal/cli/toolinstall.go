package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/drjzlyan/karya/internal/toolreg"
	"github.com/drjzlyan/karya/internal/tools"
)

// installToolIDs plans the given tool IDs from the registry (pulling in their
// dependencies in install order) and installs them into karya's isolated,
// category-based prefix via the layout dispatcher. It is the single install seam
// the language flow, `karya install`, and `karya profile` all share.
func (a *app) installToolIDs(ids []string) []tools.Result {
	plan, err := a.reg.Plan(ids)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		return nil
	}
	env := append(os.Environ(), a.env...)
	d := tools.NewLayoutDispatcher(a.paths)
	return d.Install(plan, tools.Context{Env: env, Out: os.Stdout, ErrOut: os.Stderr})
}

// refreshToolManifest republishes the resolved-tool manifest the editor reads,
// so newly installed tools resolve by absolute path on the next editor launch.
func (a *app) refreshToolManifest() {
	_ = toolreg.WriteManifest(a.paths.ToolsManifest(), a.resolver.Manifest())
}

// allResolve reports whether every given tool ID resolves in karya's prefix or
// on PATH. It is the fast-path check the launch flow uses to skip bootstrapping.
func (a *app) allResolve(ids []string) bool {
	for _, id := range ids {
		if _, ok := a.resolver.Resolve(id); !ok {
			return false
		}
	}
	return true
}

// ensureCore makes karya's launch-essential tools (tmux, Neovim) available,
// installing any that are missing into the isolated prefix via the registry
// dispatcher. It is a fast no-op when they already resolve, so it is cheap to
// call before every launch. It returns a clear error if an essential tool still
// cannot be resolved afterwards — karya's core cannot run without it.
func (a *app) ensureCore() error {
	ids := a.reg.EssentialIDs()
	if a.allResolve(ids) {
		return nil
	}
	// Provision the vendored mise first so the mise-backed core tools can install.
	if _, err := tools.EnsureMise(a.paths, os.Stdout, os.Stderr); err != nil {
		return err
	}
	a.installToolIDs(ids)
	var missing []string
	for _, id := range ids {
		if _, ok := a.resolver.Resolve(id); !ok {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("could not install %s into karya's prefix — run `karya doctor` for details",
			strings.Join(missing, ", "))
	}
	return nil
}
