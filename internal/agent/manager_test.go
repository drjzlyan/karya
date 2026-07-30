package agent

import (
	"strings"
	"testing"

	"github.com/drjzlyan/karya/internal/config"
)

func TestCycleIndex(t *testing.T) {
	tests := []struct {
		name    string
		count   int
		current int
		forward bool
		want    int
	}{
		{"next in middle", 3, 0, true, 1},
		{"next wraps", 3, 2, true, 0},
		{"prev in middle", 3, 2, false, 1},
		{"prev wraps", 3, 0, false, 2},
		{"next when current absent", 3, -1, true, 0},
		{"prev when current absent", 3, -1, false, 2},
		{"empty list", 0, 0, true, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cycleIndex(tt.count, tt.current, tt.forward); got != tt.want {
				t.Errorf("cycleIndex(%d,%d,%v) = %d, want %d",
					tt.count, tt.current, tt.forward, got, tt.want)
			}
		})
	}
}

func TestIndexOf(t *testing.T) {
	list := []string{"crush", "claude", "codex"}
	if got := indexOf(list, "codex"); got != 2 {
		t.Errorf("indexOf codex = %d, want 2", got)
	}
	if got := indexOf(list, "gemini"); got != -1 {
		t.Errorf("indexOf absent = %d, want -1", got)
	}
}

// fakeTmux records Run calls and answers Output from a canned option table.
type fakeTmux struct {
	opts  map[string]string // @ide_* option -> value
	panes string            // list-panes -F #{pane_id} output
	runs  [][]string
}

func (f *fakeTmux) Run(args ...string) error {
	f.runs = append(f.runs, args)
	// Track set-option so the Manager's writes are observable.
	if len(args) == 5 && args[0] == "set-option" {
		f.opts[args[3]] = args[4]
	}
	return nil
}

func (f *fakeTmux) Output(args ...string) (string, error) {
	switch args[0] {
	case "show-option": // show-option -t sess -v <name>
		return f.opts[args[4]], nil
	case "list-panes":
		return f.panes, nil
	}
	return "", nil
}

func (f *fakeTmux) ran(name string) bool {
	for _, r := range f.runs {
		if len(r) > 0 && r[0] == name {
			return true
		}
	}
	return false
}

// fakePrefs is an in-memory PrefStore.
type fakePrefs struct{ m map[string]string }

func newFakePrefs() *fakePrefs           { return &fakePrefs{m: map[string]string{}} }
func (p *fakePrefs) Get(k string) string { return p.m[k] }
func (p *fakePrefs) Set(k, v string) error {
	p.m[k] = v
	return nil
}
func (p *fakePrefs) Delete(k string) error {
	delete(p.m, k)
	return nil
}

func TestNextUpdatesCurrentAndSavesPref(t *testing.T) {
	ft := &fakeTmux{
		opts: map[string]string{
			"@ide_agents":        "crush claude codex",
			"@ide_current_agent": "crush",
			"@ide_workdir":       "/home/me/proj",
			"@ide_agent_pane":    "%7",
		},
		panes: "%1\n%7\n%9", // agent pane %7 still present
	}
	fp := newFakePrefs()
	m := NewManager(ft, fp, "proj", "/usr/local/bin/karya")

	if err := m.Next(); err != nil {
		t.Fatalf("Next: %v", err)
	}
	if got := ft.opts["@ide_current_agent"]; got != "claude" {
		t.Errorf("current agent = %q, want claude", got)
	}
	if got := fp.Get("agent./home/me/proj"); got != "claude" {
		t.Errorf("saved pref = %q, want claude", got)
	}
	if !ft.ran("respawn-pane") {
		t.Error("expected the agent pane to be respawned")
	}
}

func TestNextWithNoAgentsIsNoOp(t *testing.T) {
	ft := &fakeTmux{opts: map[string]string{"@ide_current_agent": "none"}}
	m := NewManager(ft, newFakePrefs(), "proj", "karya")
	m.detect = func() []string { return nil } // hermetic: no agents on PATH
	if err := m.Next(); err != nil {
		t.Fatalf("Next: %v", err)
	}
	if ft.ran("respawn-pane") {
		t.Error("respawn-pane should not run when no agents are available")
	}
}

func TestSwitchInteractiveBuildsCallback(t *testing.T) {
	ft := &fakeTmux{opts: map[string]string{
		"@ide_agents":        "crush claude",
		"@ide_current_agent": "crush",
	}}
	m := NewManager(ft, newFakePrefs(), "proj", "/usr/local/bin/karya")
	if err := m.SwitchInteractive(); err != nil {
		t.Fatalf("SwitchInteractive: %v", err)
	}
	var prompt []string
	for _, r := range ft.runs {
		if len(r) > 0 && r[0] == "command-prompt" {
			prompt = r
		}
	}
	if prompt == nil {
		t.Fatal("expected a command-prompt invocation")
	}
	callback := prompt[len(prompt)-1]
	if !strings.Contains(callback, "/usr/local/bin/karya agent switch-to %%") {
		t.Errorf("callback = %q, want it to call `agent switch-to %%%%`", callback)
	}
}

// TestRecreateDevWindowNamespacesNvim guards the isolation primitive: when the
// dev window has to be rebuilt, the editor pane must launch Neovim with the
// namespaced NVIM_APPNAME (karya/nvim), otherwise it reads the wrong config dir.
func TestRecreateDevWindowNamespacesNvim(t *testing.T) {
	ft := &fakeTmux{opts: map[string]string{"@ide_workdir": "/home/me/proj"}}
	m := NewManager(ft, newFakePrefs(), "proj", "karya")
	m.detect = func() []string { return nil } // no agent relaunch noise

	// No "dev" window exists (fake list-windows is empty) → recreateDevWindow.
	if err := m.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	want := "NVIM_APPNAME=" + config.NvimAppName + " nvim"
	var got string
	for _, r := range ft.runs {
		if len(r) >= 4 && r[0] == "send-keys" && strings.HasSuffix(r[3], " nvim") {
			got = r[3]
		}
	}
	if got != want {
		t.Errorf("editor launch = %q, want %q", got, want)
	}
}

func TestClearPref(t *testing.T) {
	ft := &fakeTmux{opts: map[string]string{"@ide_workdir": "/home/me/proj"}}
	fp := newFakePrefs()
	fp.m["agent./home/me/proj"] = "claude"
	m := NewManager(ft, fp, "proj", "karya")
	if err := m.ClearPref(); err != nil {
		t.Fatalf("ClearPref: %v", err)
	}
	if got := fp.Get("agent./home/me/proj"); got != "" {
		t.Errorf("pref not cleared: %q", got)
	}
}
