package assets

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestNvimVersionStable(t *testing.T) {
	v1, err := NvimVersion()
	if err != nil {
		t.Fatalf("NvimVersion: %v", err)
	}
	if v1 == "" {
		t.Fatal("NvimVersion returned empty string")
	}
	v2, err := NvimVersion()
	if err != nil {
		t.Fatalf("NvimVersion (2nd): %v", err)
	}
	if v1 != v2 {
		t.Errorf("NvimVersion not deterministic: %q != %q", v1, v2)
	}
}

func TestExtractNvimConfig(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "nvim")
	if err := ExtractNvimConfig(dest); err != nil {
		t.Fatalf("ExtractNvimConfig: %v", err)
	}

	// Representative files land at the root of destDir (the embed prefix is stripped).
	for _, rel := range []string{"init.lua", "lua/core/options.lua", "lazy-lock.json"} {
		got, err := os.ReadFile(filepath.Join(dest, rel))
		if err != nil {
			t.Fatalf("expected extracted %s: %v", rel, err)
		}
		want, err := fs.ReadFile(nvimFS, "nvim/"+rel)
		if err != nil {
			t.Fatalf("read embedded %s: %v", rel, err)
		}
		if string(got) != string(want) {
			t.Errorf("%s content mismatch with embedded copy", rel)
		}
	}

	// The manifest records the embedded version.
	version, err := NvimVersion()
	if err != nil {
		t.Fatalf("NvimVersion: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dest, manifestName))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if m.Version != version {
		t.Errorf("manifest version = %q, want %q", m.Version, version)
	}
}

func TestExtractCleansStaleFiles(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "nvim")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(dest, "stale.lua")
	if err := os.WriteFile(stale, []byte("-- old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ExtractNvimConfig(dest); err != nil {
		t.Fatalf("ExtractNvimConfig: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale file survived extraction (err=%v)", err)
	}
}

func TestEnsureNvimConfig(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "nvim")

	// First call extracts.
	extracted, err := EnsureNvimConfig(dest)
	if err != nil {
		t.Fatalf("EnsureNvimConfig (fresh): %v", err)
	}
	if !extracted {
		t.Fatal("expected fresh extraction on first call")
	}

	// Second call is a no-op: the manifest matches the embedded version.
	extracted, err = EnsureNvimConfig(dest)
	if err != nil {
		t.Fatalf("EnsureNvimConfig (idempotent): %v", err)
	}
	if extracted {
		t.Error("expected no re-extraction when config is current")
	}

	// A stale manifest forces re-extraction.
	if err := os.WriteFile(filepath.Join(dest, manifestName), []byte(`{"version":"stale"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	extracted, err = EnsureNvimConfig(dest)
	if err != nil {
		t.Fatalf("EnsureNvimConfig (stale): %v", err)
	}
	if !extracted {
		t.Error("expected re-extraction when manifest is stale")
	}
	// init.lua is restored and the manifest is current again.
	if _, err := os.Stat(filepath.Join(dest, "init.lua")); err != nil {
		t.Errorf("init.lua missing after re-extraction: %v", err)
	}
}
