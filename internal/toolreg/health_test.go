package toolreg

import "testing"

func TestHealthCheckerReportsInstalled(t *testing.T) {
	rv, bin := testResolver(t, nil)
	seed(t, bin, "gopls")
	h := NewHealthChecker(rv)
	h.version = func(path string, args []string) string { return "gopls v0.15.0" }

	tool, _ := rv.reg.Get("gopls")
	got := h.Check(tool)
	if !got.Installed || got.Source != SourceManaged {
		t.Errorf("expected installed/managed; got %+v", got)
	}
	if got.Version != "gopls v0.15.0" {
		t.Errorf("version = %q, want probed value", got.Version)
	}
}

func TestHealthCheckerReportsMissingWithHint(t *testing.T) {
	rv, _ := testResolver(t, nil)
	h := NewHealthChecker(rv)

	tool, _ := rv.reg.Get("gopls")
	got := h.Check(tool)
	if got.Installed || got.Source != SourceMissing {
		t.Errorf("expected missing; got %+v", got)
	}
	if got.RepairHint == "" {
		t.Error("missing tool should carry a repair hint")
	}
}

func TestHealthCheckerDetectHint(t *testing.T) {
	rv, _ := testResolver(t, nil)
	h := NewHealthChecker(rv)
	clangd, _ := rv.reg.Get("clangd")
	got := h.Check(clangd)
	if got.RepairHint != clangd.Hint {
		t.Errorf("detect tool should surface its own Hint; got %q", got.RepairHint)
	}
}

func TestHealthCheckerSkipsVersionForArtifact(t *testing.T) {
	rv, _ := testResolver(t, nil)
	seed(t, rv.paths.ToolsDir(), "lombok.jar")
	h := NewHealthChecker(rv)
	h.version = func(string, []string) string {
		t.Error("artifact-only tool should not run a version probe")
		return ""
	}
	lombok, _ := rv.reg.Get("lombok")
	got := h.Check(lombok)
	if !got.Installed || got.Version != "" {
		t.Errorf("artifact tool = %+v; want installed, no version", got)
	}
}
