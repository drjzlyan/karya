package tools

import (
	"io"
	"strings"
	"testing"
)

func TestProvisionCmdShape(t *testing.T) {
	env := []string{"MISE_DATA_DIR=/x", "MISE_TRUSTED_CONFIG_PATHS=/work/proj"}
	cmd := provisionCmd("/karya/mise", "/work/proj", env, io.Discard, io.Discard)

	if cmd.Dir != "/work/proj" {
		t.Errorf("cmd.Dir = %q, want the project root", cmd.Dir)
	}
	if len(cmd.Args) != 2 || cmd.Args[1] != "install" {
		t.Errorf("cmd.Args = %v, want [mise install]", cmd.Args)
	}
	joined := strings.Join(cmd.Env, "\n")
	if !strings.Contains(joined, "MISE_TRUSTED_CONFIG_PATHS=/work/proj") {
		t.Errorf("provision env must trust the project config; got:\n%s", joined)
	}
}
