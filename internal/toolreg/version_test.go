package toolreg

import "testing"

func TestVersionManagerQuery(t *testing.T) {
	reg := New()
	vm := &VersionManager{reg: reg, outdated: func() map[string]miseVersions {
		return map[string]miseVersions{
			"ripgrep": {current: "14.0.0", latest: "14.1.1"}, // ripgrep tool has Pkg "ripgrep"
		}
	}}
	got := vm.Query([]string{"ripgrep", "gopls"})
	if len(got) != 2 {
		t.Fatalf("Query returned %d, want 2", len(got))
	}
	rg := got[0]
	if !rg.UpdateAvailable || rg.Installed != "14.0.0" || rg.Latest != "14.1.1" {
		t.Errorf("ripgrep = %+v, want update 14.0.0->14.1.1", rg)
	}
	// gopls is a Go tool, not mise — no update info even though outdated ran.
	if got[1].UpdateAvailable || got[1].Latest != "" {
		t.Errorf("non-mise gopls should have no update info; got %+v", got[1])
	}
}

func TestVersionManagerUpdatesFilters(t *testing.T) {
	reg := New()
	vm := &VersionManager{reg: reg, outdated: func() map[string]miseVersions {
		return map[string]miseVersions{"jq": {current: "1.7", latest: "1.7.1"}}
	}}
	ups := vm.Updates([]string{"jq", "fd", "gopls"})
	if len(ups) != 1 || ups[0].ID != "jq" {
		t.Errorf("Updates = %+v, want only jq", ups)
	}
}

func TestMiseOutdatedParseSkipsHeaderAndSource(t *testing.T) {
	// Not a live mise call — exercise the version-column heuristic directly.
	if !looksLikeVersion("14.1.1") || looksLikeVersion("~/.config/mise/config.toml") {
		t.Error("looksLikeVersion heuristic wrong")
	}
	if compareVersions("14.0.0", "14.1.1") >= 0 {
		t.Error("14.0.0 should be < 14.1.1")
	}
	if compareVersions("1.7", "1.7") != 0 {
		t.Error("equal versions should compare 0")
	}
}
