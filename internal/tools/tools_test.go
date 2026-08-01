package tools

import (
	"strings"
	"testing"
)

func TestSummarize(t *testing.T) {
	got := Summarize([]Result{
		{Status: Installed}, {Status: Installed},
		{Status: Skipped}, {Status: Missing}, {Status: Failed},
	})
	for _, want := range []string{"2 installed", "1 already present", "1 need manual install", "1 failed"} {
		if !strings.Contains(got, want) {
			t.Errorf("Summarize missing %q; got %q", want, got)
		}
	}
}

func TestSafeJoinRejectsTraversal(t *testing.T) {
	base := t.TempDir()
	if _, err := safeJoin(base, "../escape"); err == nil {
		t.Error("safeJoin should reject path traversal")
	}
	if _, err := safeJoin(base, "ok/child.txt"); err != nil {
		t.Errorf("safeJoin should allow in-tree paths: %v", err)
	}
}

func TestStripComponents(t *testing.T) {
	if got := stripComponents("jdtls-1.0/plugins/a.jar", 1); got != "plugins/a.jar" {
		t.Errorf("stripComponents = %q", got)
	}
	if got := stripComponents("top", 1); got != "" {
		t.Errorf("stripComponents of shallow path = %q, want empty", got)
	}
	if got := stripComponents("/a/b", 0); got != "a/b" {
		t.Errorf("stripComponents strip 0 = %q, want a/b", got)
	}
}
