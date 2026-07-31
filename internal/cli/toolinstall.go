package cli

import (
	"fmt"
	"os"

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
