package toolreg

import "testing"

// TestProfileToolsExist guards that every profile references real catalog tools
// and that always-on IDs resolve — the membership lists are hand-written, so this
// catches drift from the registry.
func TestProfileToolsExist(t *testing.T) {
	r := New()
	for _, id := range AlwaysOnIDs() {
		if _, ok := r.Get(id); !ok {
			t.Errorf("always-on id %q is not in the registry", id)
		}
	}
	for _, p := range r.Profiles() {
		if len(p.Tools) == 0 {
			t.Errorf("profile %q has no tools", p.ID)
		}
		for _, id := range p.Tools {
			if _, ok := r.Get(id); !ok {
				t.Errorf("profile %q references unknown tool %q", p.ID, id)
			}
		}
	}
}

func TestLanguageProfileDerivedFromRegistry(t *testing.T) {
	r := New()
	got := r.LanguageIDs("go")
	want := map[string]bool{"gopls": true, "goimports": true, "delve": true}
	if len(got) != len(want) {
		t.Fatalf("LanguageIDs(go) = %v, want %d tools", got, len(want))
	}
	for _, id := range got {
		if !want[id] {
			t.Errorf("unexpected go tool %q", id)
		}
	}
}

func TestProfileLookup(t *testing.T) {
	r := New()
	python, ok := r.Profile("python")
	if !ok {
		t.Fatal("python profile missing")
	}
	if len(python.Runtimes) != 1 || python.Runtimes[0] != "python" {
		t.Errorf("python profile runtimes = %v, want [python]", python.Runtimes)
	}
	cpp, ok := r.Profile("cpp")
	if !ok || len(cpp.Runtimes) != 0 {
		t.Errorf("cpp profile should have no managed runtime; got %+v", cpp)
	}
	if _, ok := r.Profile("nope"); ok {
		t.Error("unknown profile should not be found")
	}
}
