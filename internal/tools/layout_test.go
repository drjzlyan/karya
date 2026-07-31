package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/drjzlyan/karya/internal/config"
	"github.com/drjzlyan/karya/internal/toolreg"
)

// recordDirsMethod captures the install dirs the dispatcher passes for a tool.
type recordDirsMethod struct {
	base
	gotToolsDir *string
	gotBinDir   *string
}

func (r recordDirsMethod) Available(toolreg.Tool, Context) bool { return false }
func (r recordDirsMethod) Install(_ toolreg.Tool, ctx Context) error {
	*r.gotToolsDir = ctx.ToolsDir
	*r.gotBinDir = ctx.BinDir
	return nil
}

func TestLayoutDispatcherRoutesByCategory(t *testing.T) {
	p := config.Paths{Data: t.TempDir()}
	cases := []struct {
		id      string
		loc     toolreg.InstallLocation
		wantDir string
	}{
		{"core-cli", toolreg.Core(), p.ToolCategoryDir("core")},
		{"doc-tool", toolreg.Docs(), p.ToolCategoryDir("docs")},
		{"go-tool", toolreg.Lang("go"), p.ToolCategoryDir("go")},
		{"runtime", toolreg.RuntimeAt(), p.ToolsDir()}, // no category -> shared prefix
	}
	for _, tc := range cases {
		var toolsDir, binDir string
		d := NewLayoutDispatcher(p)
		d.methods["fake"] = recordDirsMethod{gotToolsDir: &toolsDir, gotBinDir: &binDir}
		d.Install([]toolreg.Tool{{ID: tc.id, Name: tc.id, Method: "fake", Location: tc.loc}}, Context{})
		if toolsDir != tc.wantDir {
			t.Errorf("%s: ToolsDir = %q, want %q", tc.id, toolsDir, tc.wantDir)
		}
		wantBin := filepath.Join(tc.wantDir, "bin")
		if tc.wantDir == p.ToolsDir() {
			wantBin = p.ToolsBin()
		}
		if binDir != wantBin {
			t.Errorf("%s: BinDir = %q, want %q", tc.id, binDir, wantBin)
		}
	}
}

func TestLegacyDispatcherIgnoresLayout(t *testing.T) {
	var toolsDir string
	d := NewDispatcher()
	d.methods["fake"] = recordDirsMethod{gotToolsDir: &toolsDir, gotBinDir: new(string)}
	d.Install([]toolreg.Tool{{ID: "x", Name: "x", Method: "fake", Location: toolreg.Core()}},
		Context{ToolsDir: "/legacy", BinDir: "/legacy/bin"})
	if toolsDir != "/legacy" {
		t.Errorf("legacy dispatcher should use the Context dir; got %q", toolsDir)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	p := config.Paths{Data: t.TempDir(), State: t.TempDir()}
	if err := Migrate(p); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{p.DownloadsDir(), p.ToolsLogsDir()} {
		if _, err := os.Stat(d); err != nil {
			t.Errorf("Migrate did not create %q: %v", d, err)
		}
	}
	marker := filepath.Join(p.State, "layout.version")
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("layout marker not written: %v", err)
	}
	// A second run is a no-op and must not error.
	if err := Migrate(p); err != nil {
		t.Fatalf("second Migrate errored: %v", err)
	}
}
