package doctor

import (
	"testing"

	"github.com/drjzlyan/karya/internal/config"
	"github.com/drjzlyan/karya/internal/lang"
)

// healthyProbe returns a Probe where every dependency is present, so Run yields
// no Problem-level checks. Individual tests override one field to exercise a gap.
func healthyProbe() Probe {
	sel := lang.NewSelection()
	sel.Set("go", []string{"1.26"})
	return Probe{
		Paths: config.Paths{
			Config: "/home/u/.config/karya",
			Data:   "/home/u/.local/share/karya",
			State:  "/home/u/.local/state/karya",
			Cache:  "/home/u/.cache/karya",
		},
		Selection:     sel,
		KaryaVersion:  "test",
		LookPath:      func(string) (string, bool) { return "/usr/bin/x", true },
		Version:       func(string) string { return "9.9" },
		Exists:        func(string) bool { return true },
		Detected:      func() []string { return []string{"claude"} },
		ToolInstalled: func(string) bool { return true },
	}
}

func findCheck(r Report, name string) (Check, bool) {
	for _, c := range r.Checks {
		if c.Name == name {
			return c, true
		}
	}
	return Check{}, false
}

func TestHealthyReport(t *testing.T) {
	r := Run(healthyProbe())
	if !r.Healthy() {
		t.Fatalf("expected healthy report, got %d problem(s)", r.Count(Problem))
	}
	if r.Count(Warn) != 0 {
		t.Errorf("expected no warnings in a fully-healthy probe, got %d", r.Count(Warn))
	}
	if c, ok := findCheck(r, "isolation"); !ok || c.Level != OK {
		t.Errorf("isolation check = %+v, ok=%v; want OK", c, ok)
	}
}

func TestMissingEssentialToolIsProblem(t *testing.T) {
	p := healthyProbe()
	p.LookPath = func(bin string) (string, bool) {
		return "", bin != "tmux" // tmux is absent, everything else present
	}
	r := Run(p)
	if r.Healthy() {
		t.Fatal("expected a Problem when tmux is missing")
	}
	if c, _ := findCheck(r, "tmux"); c.Level != Problem {
		t.Errorf("tmux check level = %v, want Problem", c.Level)
	}
}

func TestMissingOptionalToolIsWarn(t *testing.T) {
	p := healthyProbe()
	p.LookPath = func(bin string) (string, bool) {
		return "", bin != "git" // git absent, all essentials present
	}
	r := Run(p)
	if !r.Healthy() {
		t.Fatal("a missing optional tool should not be a Problem")
	}
	if c, _ := findCheck(r, "git"); c.Level != Warn {
		t.Errorf("git check level = %v, want Warn", c.Level)
	}
}

func TestBrokenIsolationIsProblem(t *testing.T) {
	p := healthyProbe()
	p.Paths.Cache = "/tmp/not-namespaced"
	r := Run(p)
	if c, _ := findCheck(r, "isolation"); c.Level != Problem {
		t.Errorf("isolation with a non-namespaced dir = %v, want Problem", c.Level)
	}
	if r.Healthy() {
		t.Error("broken isolation should make the report unhealthy")
	}
}

func TestNoAgentsWarns(t *testing.T) {
	p := healthyProbe()
	p.Detected = func() []string { return nil }
	r := Run(p)
	if c, _ := findCheck(r, "coding agent"); c.Level != Warn {
		t.Errorf("no-agents check = %v, want Warn", c.Level)
	}
	if !r.Healthy() {
		t.Error("no agents is a Warn, not a Problem")
	}
}

func TestMissingLanguageToolingWarns(t *testing.T) {
	p := healthyProbe()
	p.ToolInstalled = func(string) bool { return false }
	r := Run(p)
	if !r.Healthy() {
		t.Error("missing editor tooling should be a Warn, not a Problem")
	}
	if r.Count(Warn) == 0 {
		t.Error("expected warnings for missing per-language tooling")
	}
	// gopls is part of the go plan; it should be reported missing.
	if c, ok := findCheck(r, "gopls"); !ok || c.Level != Warn {
		t.Errorf("gopls check = %+v, ok=%v; want Warn", c, ok)
	}
}

func TestNoLanguagesSelectedWarns(t *testing.T) {
	p := healthyProbe()
	p.Selection = lang.NewSelection()
	r := Run(p)
	if c, _ := findCheck(r, "selection"); c.Level != Warn {
		t.Errorf("empty selection check = %v, want Warn", c.Level)
	}
}
