package tools

import (
	"fmt"
	"os/exec"

	"github.com/drjzlyan/karya/internal/toolreg"
)

// miseMethod provisions a tool from mise's registry into karya's isolated
// prefix. The tool is also declared in karya's generated mise config (by the
// CLI) so its shim resolves; here we ensure the binary is downloaded and reshim
// so it lands on karya's PATH. It deliberately uses `mise install` rather than
// `mise use`, which would rewrite the generated config and be clobbered on the
// next regeneration.
type miseMethod struct{ base }

func (miseMethod) Install(t toolreg.Tool, ctx Context) error {
	mise, err := exec.LookPath("mise")
	if err != nil {
		return fmt.Errorf("mise not found — run `karya install` to provision it")
	}
	ver := t.Version
	if ver == "" {
		ver = "latest"
	}
	if err := ctx.run(nil, mise, "install", t.Pkg+"@"+ver); err != nil {
		return err
	}
	// Regenerate shims so the freshly installed tool resolves on karya's PATH.
	_ = ctx.run(nil, mise, "reshim")
	return nil
}

// uvMethod installs a Python tool with uv, redirecting its bin output to karya's
// BinDir (uv defaults to ~/.local/bin, which may not be karya's).
type uvMethod struct{ base }

func (uvMethod) Install(t toolreg.Tool, ctx Context) error {
	if _, err := exec.LookPath("uv"); err != nil {
		return fmt.Errorf("uv not found on PATH (required for Python tools)")
	}
	pkg := t.Pkg
	if t.Version != "" && t.Version != "latest" {
		pkg += "==" + t.Version
	}
	return ctx.run([]string{"UV_TOOL_BIN_DIR=" + ctx.BinDir}, "uv", "tool", "install", "--force", pkg)
}

// npmMethod installs a Node package into an isolated npm prefix (ToolsDir), so
// binaries land in ToolsDir/bin and nothing touches a global npm root.
type npmMethod struct{ base }

func (npmMethod) Install(t toolreg.Tool, ctx Context) error {
	if _, err := exec.LookPath("npm"); err != nil {
		return fmt.Errorf("npm not found on PATH")
	}
	pkg := t.Pkg
	if t.Version != "" && t.Version != "latest" {
		pkg += "@" + t.Version
	}
	return ctx.run(nil, "npm", "install", "-g", "--prefix", ctx.ToolsDir, pkg)
}

// goMethod installs a Go tool with GOBIN pinned to karya's BinDir.
type goMethod struct{ base }

func (goMethod) Install(t toolreg.Tool, ctx Context) error {
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("go not found on PATH")
	}
	return ctx.run([]string{"GOBIN=" + ctx.BinDir}, "go", "install", t.Pkg)
}

// rustupMethod adds a rustup component. This is the one tool class karya cannot
// fully isolate: components attach to the active rustup toolchain. It is
// detect-first (skipped when already present) and best-effort.
type rustupMethod struct{ base }

func (rustupMethod) Install(t toolreg.Tool, ctx Context) error {
	if _, err := exec.LookPath("rustup"); err != nil {
		return fmt.Errorf("rustup not found on PATH (install rustup for Rust tooling)")
	}
	return ctx.run(nil, "rustup", "component", "add", t.Pkg)
}

// detectMethod never installs: the dispatcher reports a missing detect tool with
// its hint. Install exists only to satisfy the Method interface.
type detectMethod struct{ base }

func (detectMethod) Install(toolreg.Tool, Context) error { return nil }
