package editor

import (
	"fmt"
	"os"
	"os/exec"
)

// syncPluginsArgs are the Neovim CLI arguments that headlessly install and update
// the editor's plugins, then quit. `Lazy! sync` is the non-interactive form so
// the process exits on its own instead of waiting on the UI.
func syncPluginsArgs() []string {
	return []string{"--headless", "+Lazy! sync", "+qa"}
}

// SyncPlugins runs Neovim headlessly to install/update the IDE's plugins into
// karya's isolated data dir. env carries the karya namespacing (NVIM_APPNAME) and
// is appended to the current environment; pass config.Paths.Env(bin).
//
// It is a deliberate no-op when nvim is not on PATH: lazy.nvim bootstraps itself
// on the first interactive editor launch, so a missing nvim here is not fatal —
// it just means plugins install lazily instead of up front. Errors from a present
// nvim (e.g. no network) are surfaced so install/update can report them.
func SyncPlugins(env []string) error {
	path, err := exec.LookPath("nvim")
	if err != nil {
		return nil
	}
	cmd := exec.Command(path, syncPluginsArgs()...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nvim plugin sync: %w", err)
	}
	return nil
}
