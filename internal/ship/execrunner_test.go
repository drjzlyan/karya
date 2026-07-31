package ship

import (
	"runtime"
	"strings"
	"testing"
)

// TestExecRunnerOutputFoldsStderr verifies a failing command surfaces its stderr
// in the returned error, not a bare "exit status 1".
func TestExecRunnerOutputFoldsStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell")
	}
	_, err := ExecRunner{}.Output("", "sh", "-c", "echo boom 1>&2; exit 3")
	if err == nil {
		t.Fatal("expected an error from a failing command")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error should include captured stderr, got: %v", err)
	}
	if strings.TrimSpace(err.Error()) == "exit status 3" {
		t.Errorf("error should be more than a bare exit status: %v", err)
	}
}

// TestExecRunnerOutputSuccess confirms the happy path still returns trimmed
// stdout with no error.
func TestExecRunnerOutputSuccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell")
	}
	out, err := ExecRunner{}.Output("", "sh", "-c", "printf 'hello\n'")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "hello" {
		t.Errorf("Output = %q, want %q", out, "hello")
	}
}
