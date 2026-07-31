package tools

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/drjzlyan/karya/internal/config"
)

// miseRepo is the GitHub project karya vendors its isolated mise from.
const miseRepo = "jdx/mise"

// EnsureMise guarantees a mise binary is available for karya's isolated
// toolchain and returns its path. If mise already resolves — in karya's own
// tool prefix or on PATH — it is returned untouched. Otherwise the latest mise
// release for this platform is downloaded into karya's ToolsBin, verified
// against the release's published SHA-256 checksum, and made executable.
//
// mise is installed only inside karya's prefix; the user's global mise,
// Homebrew, and shell are never touched. All failures return a clear,
// user-facing error — never a stack trace.
func EnsureMise(p config.Paths, out, errOut io.Writer) (string, error) {
	dest := filepath.Join(p.ToolsBin(), "mise")
	if path, ok := existingMise(dest); ok {
		return path, nil
	}
	logline(out, "mise not found — installing it into karya's isolated prefix…")

	rel, err := latestMiseRelease()
	if err != nil {
		return "", err
	}
	assetName, err := miseAssetName(rel.Tag)
	if err != nil {
		return "", err
	}
	asset, ok := findGHAsset(rel.Assets, assetName)
	if !ok {
		return "", fmt.Errorf("mise release %s has no build for %s/%s", rel.Tag, runtime.GOOS, runtime.GOARCH)
	}
	sums, ok := findGHAsset(rel.Assets, "SHASUMS256.txt")
	if !ok {
		return "", fmt.Errorf("mise release %s is missing its checksum file", rel.Tag)
	}

	tarPath, err := downloadToTemp(asset.URL, "karya-mise-*.tar.gz")
	if err != nil {
		return "", fmt.Errorf("could not download mise: %w", err)
	}
	defer cleanup(tarPath)

	sumText, err := fetchText(sums.URL)
	if err != nil {
		return "", fmt.Errorf("could not download mise checksums: %w", err)
	}
	want, ok := parseShasums(sumText)[assetName]
	if !ok {
		return "", fmt.Errorf("mise release %s has no checksum for %s", rel.Tag, assetName)
	}
	got, err := sha256File(tarPath)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(got, want) {
		return "", fmt.Errorf("mise download failed its integrity check (expected %s, got %s) — please retry", want, got)
	}

	if err := os.MkdirAll(p.ToolsBin(), 0o755); err != nil {
		return "", fmt.Errorf("could not create karya's tool directory: %w", err)
	}
	if err := extractMiseBinary(tarPath, dest); err != nil {
		return "", err
	}
	logline(out, "installed mise "+rel.Tag)
	return dest, nil
}

// existingMise resolves a usable mise: karya's vendored copy first, then PATH.
func existingMise(dest string) (string, bool) {
	if st, err := os.Stat(dest); err == nil && !st.IsDir() {
		return dest, true
	}
	if path, err := exec.LookPath("mise"); err == nil {
		return path, true
	}
	return "", false
}

// ── GitHub release resolution ───────────────────────────────────────────────

type ghAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type ghRelease struct {
	Tag    string    `json:"tag_name"`
	Assets []ghAsset `json:"assets"`
}

// latestMiseRelease queries GitHub for mise's latest release metadata.
func latestMiseRelease() (ghRelease, error) {
	url := "https://api.github.com/repos/" + miseRepo + "/releases/latest"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return ghRelease{}, fmt.Errorf("could not reach GitHub to find mise: %w", err)
	}
	req.Header.Set("User-Agent", "karya")
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return ghRelease{}, fmt.Errorf("could not reach GitHub to find mise — check your network: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ghRelease{}, fmt.Errorf("could not fetch the latest mise release (GitHub returned %s)", resp.Status)
	}
	var rel ghRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&rel); err != nil {
		return ghRelease{}, fmt.Errorf("could not read the mise release list: %w", err)
	}
	if rel.Tag == "" {
		return ghRelease{}, fmt.Errorf("GitHub returned no usable mise release")
	}
	return rel, nil
}

// miseAssetName is the release tarball name for this platform, e.g.
// "mise-v2026.7.18-macos-arm64.tar.gz".
func miseAssetName(tag string) (string, error) {
	os2, arch, err := misePlatform()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("mise-%s-%s-%s.tar.gz", tag, os2, arch), nil
}

// misePlatform maps Go's GOOS/GOARCH to mise's release naming.
func misePlatform() (osName, arch string, err error) {
	switch runtime.GOOS {
	case "darwin":
		osName = "macos"
	case "linux":
		osName = "linux"
	default:
		return "", "", fmt.Errorf("karya's dependency installer does not support %s yet", runtime.GOOS)
	}
	switch runtime.GOARCH {
	case "amd64":
		arch = "x64"
	case "arm64":
		arch = "arm64"
	default:
		return "", "", fmt.Errorf("karya's dependency installer does not support %s/%s yet", runtime.GOOS, runtime.GOARCH)
	}
	return osName, arch, nil
}

func findGHAsset(assets []ghAsset, name string) (ghAsset, bool) {
	for _, a := range assets {
		if a.Name == name {
			return a, true
		}
	}
	return ghAsset{}, false
}

// parseShasums parses a "SHASUMS256.txt" body ("<hex>  <filename>" per line)
// into a filename→hash map. It normalizes the leading "./" (and the "*" binary
// marker) that coreutils-style checksum files prepend to the filename.
func parseShasums(text string) map[string]string {
	sums := make(map[string]string)
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		name := strings.TrimPrefix(strings.TrimPrefix(fields[1], "*"), "./")
		sums[name] = fields[0]
	}
	return sums
}

// ── download + extract ──────────────────────────────────────────────────────

// downloadToTemp streams url into a temp file and returns its path. The caller
// is responsible for removing it (see cleanup).
func downloadToTemp(url, pattern string) (string, error) {
	tmp, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	if err := download(url, tmp); err != nil {
		tmp.Close()
		cleanup(tmp.Name())
		return "", err
	}
	if err := tmp.Close(); err != nil {
		cleanup(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
}

// sha256File returns the lowercase hex SHA-256 of the file at path.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// extractMiseBinary pulls the mise executable out of a mise release tarball
// (whose layout is mise/bin/mise) and writes it to dest with 0755.
func extractMiseBinary(tarPath, dest string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("mise download was not a valid archive: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("could not read the mise archive: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg || filepath.Base(hdr.Name) != "mise" {
			continue
		}
		if err := writeFileFrom(tr, dest, 0o755); err != nil {
			return fmt.Errorf("could not install mise: %w", err)
		}
		return nil
	}
	return fmt.Errorf("the mise archive did not contain a mise binary")
}

// logline writes a progress line when out is set.
func logline(out io.Writer, msg string) {
	if out != nil {
		fmt.Fprintln(out, msg)
	}
}
