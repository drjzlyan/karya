package tools

import (
	"testing"

	"github.com/drjzlyan/karya/internal/toolreg"
)

// fakeMethod records install attempts and reports a fixed availability, so
// dispatch behavior can be verified without touching the network or PATH.
type fakeMethod struct {
	base
	avail     bool
	installed *[]string
	fail      error
}

func (f fakeMethod) Available(toolreg.Tool, Context) bool { return f.avail }
func (f fakeMethod) Install(t toolreg.Tool, _ Context) error {
	*f.installed = append(*f.installed, t.ID)
	return f.fail
}

func TestDispatcherInstallsMissingInOrder(t *testing.T) {
	var installed []string
	d := &Dispatcher{methods: map[toolreg.InstallMethod]Method{
		"fake": fakeMethod{avail: false, installed: &installed},
	}}
	got := d.Install([]toolreg.Tool{
		{ID: "a", Name: "a", Method: "fake"},
		{ID: "b", Name: "b", Method: "fake"},
	}, Context{})
	if len(installed) != 2 || installed[0] != "a" || installed[1] != "b" {
		t.Fatalf("expected a,b installed in order; got %v", installed)
	}
	for _, r := range got {
		if r.Status != Installed {
			t.Errorf("%s: status %v, want Installed", r.Tool, r.Status)
		}
	}
}

func TestDispatcherSkipsAvailable(t *testing.T) {
	var installed []string
	d := &Dispatcher{methods: map[toolreg.InstallMethod]Method{
		"fake": fakeMethod{avail: true, installed: &installed},
	}}
	got := d.Install([]toolreg.Tool{{ID: "a", Name: "a", Method: "fake"}}, Context{})
	if len(installed) != 0 {
		t.Errorf("available tool should not be installed; got %v", installed)
	}
	if got[0].Status != Skipped {
		t.Errorf("status %v, want Skipped", got[0].Status)
	}
}

func TestDispatcherUnknownMethodFails(t *testing.T) {
	d := &Dispatcher{methods: map[toolreg.InstallMethod]Method{}}
	got := d.Install([]toolreg.Tool{{ID: "a", Name: "a", Method: "nope"}}, Context{})
	if got[0].Status != Failed || got[0].Err == nil {
		t.Errorf("unknown method should Fail with an error; got %+v", got[0])
	}
}

func TestDispatcherDetectMissingReportsHint(t *testing.T) {
	// The real detect method is registered by NewDispatcher; an absent binary
	// must report Missing rather than attempt an install.
	d := NewDispatcher()
	got := d.Install([]toolreg.Tool{{
		ID: "clangd", Name: "clangd", Method: toolreg.MethodDetect,
		Executable: "definitely-not-a-real-binary-xyz", Hint: "install it",
	}}, Context{ToolsDir: t.TempDir(), BinDir: t.TempDir()})
	if got[0].Status != Missing {
		t.Errorf("missing detect tool should be Missing; got %v", got[0].Status)
	}
}
