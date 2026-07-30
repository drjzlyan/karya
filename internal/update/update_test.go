package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestIsNewer(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"v0.1.0", "v0.2.0", true},
		{"v0.2.0", "v0.2.0", false},
		{"v0.2.0", "v0.1.0", false},
		{"v0.1.0", "v0.1.1", true},
		{"v1.0.0", "v1.0.0", false},
		{"v0.9.0", "v0.10.0", true}, // numeric, not lexical
		{"dev", "v0.1.0", true},     // dev always updatable
		{"", "v0.1.0", true},
		{"v1.2.0-rc1", "v1.2.0", false}, // suffix on field ignored → equal
		{"0.1.0", "0.2.0", true},        // missing leading v tolerated
	}
	for _, c := range cases {
		if got := IsNewer(c.current, c.latest); got != c.want {
			t.Errorf("IsNewer(%q,%q)=%v want %v", c.current, c.latest, got, c.want)
		}
	}
}

func TestAssetName(t *testing.T) {
	u := Updater{OS: "darwin", Arch: "arm64"}
	if got, want := u.AssetName("v0.2.0"), "karya_0.2.0_darwin_arm64.tar.gz"; got != want {
		t.Errorf("AssetName=%q want %q", got, want)
	}
	// A tag without a leading v should still produce a v-less filename.
	if got, want := u.AssetName("0.2.0"), "karya_0.2.0_darwin_arm64.tar.gz"; got != want {
		t.Errorf("AssetName=%q want %q", got, want)
	}
}

func TestParseChecksums(t *testing.T) {
	data := []byte("abc123  karya_0.2.0_darwin_arm64.tar.gz\ndef456  karya_0.2.0_linux_amd64.tar.gz\n\n# comment line ignored\n")
	got := parseChecksums(data)
	if got["karya_0.2.0_darwin_arm64.tar.gz"] != "abc123" {
		t.Errorf("darwin checksum = %q", got["karya_0.2.0_darwin_arm64.tar.gz"])
	}
	if got["karya_0.2.0_linux_amd64.tar.gz"] != "def456" {
		t.Errorf("linux checksum = %q", got["karya_0.2.0_linux_amd64.tar.gz"])
	}
	if len(got) != 2 {
		t.Errorf("parsed %d entries, want 2", len(got))
	}
}

func TestVerifySHA256(t *testing.T) {
	data := []byte("hello karya")
	sum := sha256.Sum256(data)
	good := hex.EncodeToString(sum[:])
	if err := verifySHA256(data, good); err != nil {
		t.Errorf("verifySHA256 with correct digest: %v", err)
	}
	if err := verifySHA256(data, "deadbeef"); err == nil {
		t.Error("verifySHA256 should reject a wrong digest")
	}
	// Case-insensitive match.
	if err := verifySHA256(data, upper(good)); err != nil {
		t.Errorf("verifySHA256 should be case-insensitive: %v", err)
	}
}

func TestBinaryFromTarGz(t *testing.T) {
	want := []byte("#!/bin/echo fake karya binary\n")
	archive := makeTarGz(t, map[string][]byte{binaryName: want})
	got, err := binaryFromTarGz(archive)
	if err != nil {
		t.Fatalf("binaryFromTarGz: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("extracted %q want %q", got, want)
	}
}

func TestBinaryFromTarGzNestedDir(t *testing.T) {
	want := []byte("nested binary")
	archive := makeTarGz(t, map[string][]byte{"dist/" + binaryName: want, "README.md": []byte("x")})
	got, err := binaryFromTarGz(archive)
	if err != nil {
		t.Fatalf("binaryFromTarGz: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("extracted %q want %q", got, want)
	}
}

func TestBinaryFromTarGzMissing(t *testing.T) {
	archive := makeTarGz(t, map[string][]byte{"LICENSE": []byte("mit")})
	if _, err := binaryFromTarGz(archive); err == nil {
		t.Error("expected error when archive has no karya binary")
	}
}

func TestBinaryFromTarGzTraversalRejected(t *testing.T) {
	archive := makeTarGz(t, map[string][]byte{"../../" + binaryName: []byte("evil")})
	if _, err := binaryFromTarGz(archive); err == nil {
		t.Error("expected traversal path to be rejected")
	}
}

func TestApplyAtomicReplace(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "karya")
	if err := os.WriteFile(dest, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	newBin := []byte("brand new binary")
	if err := Apply(dest, newBin); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, newBin) {
		t.Errorf("after Apply file = %q want %q", got, newBin)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("replaced binary is not executable: %v", info.Mode())
	}
	// No temp files should linger in the directory.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("expected only the destination file, got %d entries", len(entries))
	}
}

// TestLatestAndFetch drives the full network flow against an httptest server that
// mimics the GitHub releases API and asset downloads.
func TestLatestAndFetch(t *testing.T) {
	binData := []byte("real karya binary payload")
	archive := makeTarGz(t, map[string][]byte{"karya_0.2.0_" + goOS() + "_" + goArch() + "/" + binaryName: binData})
	sum := sha256.Sum256(archive)
	assetName := fmt.Sprintf("karya_0.2.0_%s_%s.tar.gz", goOS(), goArch())
	checksums := []byte(fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), assetName))

	mux := http.NewServeMux()
	var base string
	mux.HandleFunc("/repos/acme/karya/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request missing User-Agent")
		}
		rel := Release{
			Tag: "v0.2.0",
			Assets: []Asset{
				{Name: assetName, URL: base + "/dl/" + assetName},
				{Name: checksumsName, URL: base + "/dl/" + checksumsName},
			},
		}
		_ = json.NewEncoder(w).Encode(rel)
	})
	mux.HandleFunc("/dl/"+assetName, func(w http.ResponseWriter, r *http.Request) { w.Write(archive) })
	mux.HandleFunc("/dl/"+checksumsName, func(w http.ResponseWriter, r *http.Request) { w.Write(checksums) })

	srv := httptest.NewServer(mux)
	defer srv.Close()
	base = srv.URL

	u := Updater{Repo: "acme/karya", Current: "v0.1.0", OS: goOS(), Arch: goArch(), APIBase: srv.URL, Client: srv.Client()}
	rel, err := u.Latest()
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if rel.Tag != "v0.2.0" {
		t.Fatalf("tag = %q want v0.2.0", rel.Tag)
	}
	if !u.UpdateAvailable(rel) {
		t.Fatal("expected update to be available")
	}
	got, err := u.FetchBinary(rel)
	if err != nil {
		t.Fatalf("FetchBinary: %v", err)
	}
	if !bytes.Equal(got, binData) {
		t.Errorf("fetched binary = %q want %q", got, binData)
	}
}

func TestFetchBinaryChecksumMismatch(t *testing.T) {
	archive := makeTarGz(t, map[string][]byte{binaryName: []byte("payload")})
	assetName := fmt.Sprintf("karya_0.2.0_%s_%s.tar.gz", goOS(), goArch())
	// Deliberately wrong checksum.
	checksums := []byte("0000000000000000000000000000000000000000000000000000000000000000  " + assetName + "\n")

	mux := http.NewServeMux()
	mux.HandleFunc("/dl/"+assetName, func(w http.ResponseWriter, r *http.Request) { w.Write(archive) })
	mux.HandleFunc("/dl/"+checksumsName, func(w http.ResponseWriter, r *http.Request) { w.Write(checksums) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	u := Updater{Repo: "acme/karya", Current: "v0.1.0", OS: goOS(), Arch: goArch(), Client: srv.Client()}
	rel := Release{Tag: "v0.2.0", Assets: []Asset{
		{Name: assetName, URL: srv.URL + "/dl/" + assetName},
		{Name: checksumsName, URL: srv.URL + "/dl/" + checksumsName},
	}}
	if _, err := u.FetchBinary(rel); err == nil {
		t.Error("expected checksum mismatch error")
	}
}

func TestFetchBinaryUnsupportedPlatform(t *testing.T) {
	u := Updater{Repo: "acme/karya", Current: "v0.1.0", OS: "plan9", Arch: "sparc"}
	rel := Release{Tag: "v0.2.0", Assets: []Asset{{Name: "karya_0.2.0_darwin_arm64.tar.gz"}}}
	if _, err := u.FetchBinary(rel); err == nil {
		t.Error("expected error for platform with no asset")
	}
}

// --- helpers ---

func makeTarGz(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for name, data := range files {
		hdr := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(data)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func upper(s string) string { return string(bytes.ToUpper([]byte(s))) }

// goOS/goArch keep the test platform-agnostic while matching AssetName's format.
func goOS() string   { return runtime.GOOS }
func goArch() string { return runtime.GOARCH }
