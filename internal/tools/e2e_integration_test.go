//go:build integration

package tools_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/drjzlyan/karya/internal/config"
	"github.com/drjzlyan/karya/internal/doctor"
	"github.com/drjzlyan/karya/internal/toolreg"
	"github.com/drjzlyan/karya/internal/tools"
)

// fakeMise writes an executable `mise` stub into dir and prepends dir to PATH,
// so the real dispatcher/resolver/version wiring can be exercised end-to-end with
// no network. The stub creates a shim under $MISE_DATA_DIR/shims on `install`
// (mapping the mise package name to its executable), echoes a version from each
// shim, resolves `which`, and reports a canned `outdated` row for jq.
func fakeMise(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	script := `#!/usr/bin/env bash
set -e
cmd="$1"; shift || true
case "$cmd" in
  install)
    spec="$1"; pkg="${spec%@*}"
    case "$pkg" in
      neovim) exe=nvim ;;
      *)      exe="$pkg" ;;
    esac
    mkdir -p "$MISE_DATA_DIR/shims"
    printf '#!/usr/bin/env bash\necho "%s 1.7"\n' "$exe" > "$MISE_DATA_DIR/shims/$exe"
    chmod +x "$MISE_DATA_DIR/shims/$exe"
    ;;
  reshim) : ;;
  which) echo "$MISE_DATA_DIR/shims/$1" ;;
  outdated)
    echo "Tool Requested Current Latest"
    echo "jq 1.7 1.7 1.7.1"
    ;;
  *) : ;;
esac
`
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available for the fake mise stub")
	}
	path := filepath.Join(dir, "mise")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	// Prepend the fake mise ahead of any real one; keep the rest of PATH so the
	// stub's mkdir/chmod/bash resolve.
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func newPrefix(t *testing.T) config.Paths {
	t.Helper()
	return config.Paths{Data: t.TempDir(), Config: t.TempDir(), State: t.TempDir(), Cache: t.TempDir()}
}

// TestE2EInstallResolveHealthUpdate drives the real registry → layout dispatcher
// → resolver → health → version-manager → doctor flow against the fake mise.
func TestE2EInstallResolveHealthUpdate(t *testing.T) {
	fakeMise(t)
	p := newPrefix(t)
	reg := toolreg.New()
	env := append(os.Environ(), p.MiseEnv()...)

	// Force the install through the fake mise (Reinstall bypasses the detect-first
	// skip, so the test does not depend on whether jq exists on the host PATH).
	jq, _ := reg.Get("jq")
	ctx := tools.Context{Env: env, Out: os.Stdout, ErrOut: os.Stderr}
	if res := tools.NewLayoutDispatcher(p).Reinstall(jq, ctx); res.Status != tools.Installed {
		t.Fatalf("install jq: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(p.MiseShims(), "jq")); err != nil {
		t.Fatalf("jq shim not created: %v", err)
	}

	rv := toolreg.NewResolver(p, reg)
	got, ok := rv.Resolve("jq")
	if !ok || got.Source != toolreg.SourceManaged {
		t.Fatalf("Resolve(jq) = %+v ok=%v; want managed", got, ok)
	}

	if hs := toolreg.NewHealthChecker(rv).Check(jq); !hs.Installed || !strings.Contains(hs.Version, "1.7") {
		t.Errorf("health = %+v; want installed with a version", hs)
	}

	vm := toolreg.NewVersionManager(reg, env)
	ups := vm.Updates([]string{"jq"})
	if len(ups) != 1 || !ups[0].UpdateAvailable || ups[0].Latest != "1.7.1" {
		t.Fatalf("Updates(jq) = %+v; want jq 1.7 -> 1.7.1", ups)
	}

	report := doctor.Run(doctor.Probe{Paths: p, KaryaVersion: "test", Registry: reg, CheckUpdates: true})
	var found bool
	for _, c := range report.Checks {
		if c.Group == "updates" && c.Name == "jq" {
			found = true
		}
	}
	if !found {
		t.Error("doctor --check-updates should surface the jq update")
	}
}

// TestE2ECoreBootstrapInstall covers the launch-bootstrap mechanic (Phase 2 fold):
// installing the essential tools through the same dispatcher and resolving them,
// including the neovim package -> nvim executable mapping.
func TestE2ECoreBootstrapInstall(t *testing.T) {
	fakeMise(t)
	p := newPrefix(t)
	reg := toolreg.New()
	env := append(os.Environ(), p.MiseEnv()...)

	ctx := tools.Context{Env: env, Out: os.Stdout, ErrOut: os.Stderr}
	d := tools.NewLayoutDispatcher(p)
	for _, id := range reg.EssentialIDs() {
		tool, _ := reg.Get(id)
		if res := d.Reinstall(tool, ctx); res.Status != tools.Installed {
			t.Fatalf("install essential %q: %+v", id, res)
		}
	}

	rv := toolreg.NewResolver(p, reg)
	for _, id := range reg.EssentialIDs() {
		if got, ok := rv.Resolve(id); !ok || got.Source != toolreg.SourceManaged {
			t.Errorf("essential %q did not resolve as managed after install: %+v ok=%v", id, got, ok)
		}
	}
}
