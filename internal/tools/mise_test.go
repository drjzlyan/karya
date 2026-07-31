package tools

import (
	"runtime"
	"strings"
	"testing"
)

func TestParseShasums(t *testing.T) {
	body := "abc123  ./mise-v1-macos-arm64.tar.gz\n" + // coreutils "./" prefix
		"def456  *mise-v1-linux-x64.tar.gz\n" + // binary-mode "*" marker
		"\n" + // blank line ignored
		"garbage-without-two-fields\n"
	sums := parseShasums(body)
	if got := sums["mise-v1-macos-arm64.tar.gz"]; got != "abc123" {
		t.Errorf("macos-arm64 = %q, want abc123", got)
	}
	if got := sums["mise-v1-linux-x64.tar.gz"]; got != "def456" {
		t.Errorf("linux-x64 = %q, want def456", got)
	}
	if len(sums) != 2 {
		t.Errorf("parsed %d entries, want 2: %v", len(sums), sums)
	}
}

func TestMiseAssetName(t *testing.T) {
	// The current platform must be supported (darwin/linux on amd64/arm64).
	name, err := miseAssetName("v2026.7.18")
	if err != nil {
		t.Fatalf("miseAssetName on %s/%s: %v", runtime.GOOS, runtime.GOARCH, err)
	}
	if !strings.HasPrefix(name, "mise-v2026.7.18-") || !strings.HasSuffix(name, ".tar.gz") {
		t.Errorf("unexpected asset name %q", name)
	}
	// mise uses macos/x64, never darwin/amd64.
	if strings.Contains(name, "darwin") || strings.Contains(name, "amd64") {
		t.Errorf("asset name leaks Go platform naming: %q", name)
	}
}

func TestMisePlatformArchMapping(t *testing.T) {
	if runtime.GOARCH == "amd64" {
		if _, arch, err := misePlatform(); err != nil || arch != "x64" {
			t.Errorf("amd64 => arch %q err %v, want x64", arch, err)
		}
	}
	if runtime.GOOS == "darwin" {
		if os2, _, err := misePlatform(); err != nil || os2 != "macos" {
			t.Errorf("darwin => os %q err %v, want macos", os2, err)
		}
	}
}

func TestFindGHAsset(t *testing.T) {
	assets := []ghAsset{{Name: "a.txt", URL: "u1"}, {Name: "b.txt", URL: "u2"}}
	if a, ok := findGHAsset(assets, "b.txt"); !ok || a.URL != "u2" {
		t.Errorf("findGHAsset(b.txt) = %+v %v", a, ok)
	}
	if _, ok := findGHAsset(assets, "missing"); ok {
		t.Error("findGHAsset(missing) should be false")
	}
}

func TestOnPathMissing(t *testing.T) {
	if onPath("karya-definitely-not-a-real-binary-zzz") {
		t.Error("onPath reported a nonexistent binary as available")
	}
}
