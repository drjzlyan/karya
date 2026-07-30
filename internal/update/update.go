// Package update implements karya's self-update. It queries GitHub Releases for
// the latest version, downloads the platform archive plus checksums, verifies the
// SHA-256, and atomically replaces the running binary (write to a temp file in the
// destination directory, then rename over the original).
//
// Every network and filesystem side effect sits behind a small seam so the pure
// logic — version comparison, asset naming, checksum parsing/verification, archive
// extraction, and the atomic replace — is unit-tested hermetically without a
// network or a real GitHub release. This keeps self-update honest with the
// isolation model: it only ever writes the karya binary and karya-owned dirs.
package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// defaultAPIBase is the GitHub REST API root. Tests override Updater.APIBase to
// point at an httptest server so no real network call is made.
const defaultAPIBase = "https://api.github.com"

// binaryName is the name of the karya executable inside a release archive.
const binaryName = "karya"

// checksumsName is the checksums asset goreleaser publishes for every release.
const checksumsName = "checksums.txt"

// maxDownloadBytes bounds every downloaded asset to guard against a hostile or
// corrupt release serving an unbounded body.
const maxDownloadBytes = 256 << 20 // 256 MiB

// Updater performs self-update for a single platform. The zero value is not
// usable; set at least Repo, Current, OS, and Arch.
type Updater struct {
	// Repo is the "owner/name" GitHub repository publishing releases.
	Repo string
	// Current is the running binary's version (e.g. "v0.1.0" or "dev").
	Current string
	// OS and Arch identify the platform asset to fetch (GOOS/GOARCH).
	OS, Arch string
	// APIBase overrides the GitHub API root; empty uses the public API.
	APIBase string
	// Client is the HTTP client used for all requests; nil uses a default with a
	// sane timeout.
	Client *http.Client
}

// Release is the subset of a GitHub release payload karya needs.
type Release struct {
	Tag    string  `json:"tag_name"`
	Assets []Asset `json:"assets"`
}

// Asset is one downloadable file attached to a release.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// client returns the configured HTTP client or a default one.
func (u Updater) client() *http.Client {
	if u.Client != nil {
		return u.Client
	}
	return &http.Client{Timeout: 60 * time.Second}
}

// apiBase returns the configured API root or the public default.
func (u Updater) apiBase() string {
	if u.APIBase != "" {
		return strings.TrimRight(u.APIBase, "/")
	}
	return defaultAPIBase
}

// Latest fetches the most recent published release for the repository.
func (u Updater) Latest() (Release, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", u.apiBase(), u.Repo)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return Release{}, err
	}
	// GitHub requires a User-Agent and recommends this Accept header.
	req.Header.Set("User-Agent", "karya-self-update")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := u.client().Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("query latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("query latest release: unexpected status %s", resp.Status)
	}
	var rel Release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rel); err != nil {
		return Release{}, fmt.Errorf("decode release: %w", err)
	}
	if rel.Tag == "" {
		return Release{}, fmt.Errorf("release has no tag_name")
	}
	return rel, nil
}

// UpdateAvailable reports whether rel is newer than the running binary.
func (u Updater) UpdateAvailable(rel Release) bool {
	return IsNewer(u.Current, rel.Tag)
}

// AssetName is the archive name goreleaser publishes for this platform and
// version. goreleaser's {{ .Version }} is the tag without a leading "v", so the
// tag "v0.2.0" yields "karya_0.2.0_darwin_arm64.tar.gz".
func (u Updater) AssetName(tag string) string {
	return fmt.Sprintf("%s_%s_%s_%s.tar.gz", binaryName, strings.TrimPrefix(tag, "v"), u.OS, u.Arch)
}

// FetchBinary downloads the platform archive and checksums for rel, verifies the
// archive's SHA-256, and returns the extracted karya binary bytes.
func (u Updater) FetchBinary(rel Release) ([]byte, error) {
	want := u.AssetName(rel.Tag)
	archiveAsset, ok := findAsset(rel.Assets, want)
	if !ok {
		return nil, fmt.Errorf("release %s has no asset %q (unsupported platform %s/%s?)", rel.Tag, want, u.OS, u.Arch)
	}
	sumsAsset, ok := findAsset(rel.Assets, checksumsName)
	if !ok {
		return nil, fmt.Errorf("release %s has no %s", rel.Tag, checksumsName)
	}

	archive, err := u.download(archiveAsset.URL)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", want, err)
	}
	sums, err := u.download(sumsAsset.URL)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", checksumsName, err)
	}

	wantSum, ok := parseChecksums(sums)[want]
	if !ok {
		return nil, fmt.Errorf("no checksum for %s in %s", want, checksumsName)
	}
	if err := verifySHA256(archive, wantSum); err != nil {
		return nil, err
	}
	bin, err := binaryFromTarGz(archive)
	if err != nil {
		return nil, err
	}
	return bin, nil
}

// Apply atomically replaces the file at destPath with binary. It writes to a temp
// file in the same directory (so the rename stays on one filesystem and is
// atomic), makes it executable, then renames over the destination. A running
// binary can be replaced this way on macOS and Linux because the rename swaps the
// directory entry rather than rewriting the open file.
func Apply(destPath string, binary []byte) error {
	dir := filepath.Dir(destPath)
	tmp, err := os.CreateTemp(dir, ".karya-update-*")
	if err != nil {
		return fmt.Errorf("create temp binary: %w", err)
	}
	tmpName := tmp.Name()
	// Best-effort cleanup if we fail before the rename; the rename removes it on
	// success so a later Remove is a harmless no-op.
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(binary); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp binary: %w", err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod temp binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp binary: %w", err)
	}
	if err := os.Rename(tmpName, destPath); err != nil {
		return fmt.Errorf("replace %s: %w", destPath, err)
	}
	return nil
}

// download GETs url and returns its body, bounded by maxDownloadBytes.
func (u Updater) download(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "karya-self-update")
	resp, err := u.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxDownloadBytes))
}

// findAsset returns the asset with the given name.
func findAsset(assets []Asset, name string) (Asset, bool) {
	for _, a := range assets {
		if a.Name == name {
			return a, true
		}
	}
	return Asset{}, false
}

// parseChecksums parses a goreleaser checksums.txt ("<hex>  <filename>" per line)
// into a filename→hex map.
func parseChecksums(data []byte) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		out[fields[1]] = strings.ToLower(fields[0])
	}
	return out
}

// verifySHA256 checks that data hashes to the expected lowercase hex digest.
func verifySHA256(data []byte, want string) error {
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("checksum mismatch: got %s, want %s", got, want)
	}
	return nil
}

// binaryFromTarGz extracts the karya binary from a gzip-compressed tar archive.
// It matches by base name so it is agnostic to any leading directory, and rejects
// entries whose names attempt path traversal.
func binaryFromTarGz(data []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("open gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read archive: %w", err)
		}
		if strings.Contains(hdr.Name, "..") {
			return nil, fmt.Errorf("unsafe archive path %q", hdr.Name)
		}
		if hdr.Typeflag != tar.TypeReg || path.Base(hdr.Name) != binaryName {
			continue
		}
		bin, err := io.ReadAll(io.LimitReader(tr, maxDownloadBytes))
		if err != nil {
			return nil, fmt.Errorf("read %s from archive: %w", binaryName, err)
		}
		return bin, nil
	}
	return nil, fmt.Errorf("archive contains no %s binary", binaryName)
}

// IsNewer reports whether latest is a newer version than current. A "dev" (or any
// unparseable) current is always treated as older so developers can update to a
// tagged release.
func IsNewer(current, latest string) bool {
	if current == "" || current == "dev" {
		return true
	}
	return compareVersions(latest, current) > 0
}

// compareVersions compares two dotted versions field-by-field numerically,
// ignoring a leading "v" and any pre-release/build suffix on each field. It
// returns -1, 0, or 1 for a<b, a==b, a>b.
func compareVersions(a, b string) int {
	fa := versionFields(a)
	fb := versionFields(b)
	for i := 0; i < len(fa) || i < len(fb); i++ {
		var x, y int
		if i < len(fa) {
			x = fa[i]
		}
		if i < len(fb) {
			y = fb[i]
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

// versionFields splits a version like "v1.2.3-rc1" into [1, 2, 3], parsing the
// leading integer of each dot-separated component and stopping a component at its
// first non-digit.
func versionFields(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		digits := p
		for i, r := range p {
			if r < '0' || r > '9' {
				digits = p[:i]
				break
			}
		}
		n, _ := strconv.Atoi(digits)
		out = append(out, n)
	}
	return out
}
