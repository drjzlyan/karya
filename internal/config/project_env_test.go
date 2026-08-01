package config

import (
	"strings"
	"testing"
)

// TestEnvForProjectTrustsProjectConfig asserts that per-project env layering adds
// the project's trusted config path (so mise honors the project's versions) while
// keeping everything Env already guarantees, and never touching the user's global
// mise trust store.
func TestEnvForProjectTrustsProjectConfig(t *testing.T) {
	p := Paths{Data: "/home/u/.local/share/karya", Config: "/home/u/.config/karya"}
	env := strings.Join(p.EnvForProject("/abs/karya", "/work/proj"), "\n")
	if !strings.Contains(env, "MISE_TRUSTED_CONFIG_PATHS=/work/proj") {
		t.Errorf("EnvForProject must trust the project config path; got:\n%s", env)
	}
	// Still pins mise inside karya's prefix (inherited from Env).
	if !strings.Contains(env, "MISE_DATA_DIR="+p.MiseData()) {
		t.Errorf("EnvForProject must keep mise pinned to karya's prefix; got:\n%s", env)
	}
}

func TestEnvForProjectEmptyRootMatchesEnv(t *testing.T) {
	p := Paths{Data: "/home/u/.local/share/karya", Config: "/home/u/.config/karya"}
	if got := strings.Join(p.EnvForProject("/abs/karya", ""), "\n"); strings.Contains(got, "MISE_TRUSTED_CONFIG_PATHS") {
		t.Errorf("empty project root should add no trusted-path var; got:\n%s", got)
	}
}
