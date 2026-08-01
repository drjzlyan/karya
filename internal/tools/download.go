package tools

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/drjzlyan/karya/internal/toolreg"
)

// httpClient bounds download time so a hung mirror does not stall `karya lang`.
var httpClient = &http.Client{Timeout: 5 * time.Minute}

// jdtlsMethod downloads and unpacks the Eclipse JDT language server into
// ToolsDir/jdtls-<version>, points ToolsDir/jdtls at it, and writes a launcher
// wrapper into BinDir so it resolves on karya's PATH.
type jdtlsMethod struct{ base }

func (jdtlsMethod) Install(t toolreg.Tool, ctx Context) error {
	target := filepath.Join(ctx.ToolsDir, "jdtls-"+t.Version)
	base := "https://download.eclipse.org/jdtls/milestones/" + t.Version

	if _, err := os.Stat(filepath.Join(target, "plugins")); err != nil {
		name, err := fetchText(base + "/latest.txt")
		if err != nil {
			return fmt.Errorf("resolve jdtls tarball: %w", err)
		}
		name = strings.TrimSpace(name)
		if name == "" {
			return fmt.Errorf("empty jdtls tarball name for %s", t.Version)
		}
		tmp, err := os.CreateTemp("", "jdtls-*.tar.gz")
		if err != nil {
			return err
		}
		defer cleanup(tmp.Name())
		if err := download(base+"/"+name, tmp); err != nil {
			return err
		}
		if err := os.MkdirAll(target, 0o755); err != nil {
			return err
		}
		if err := extractTarGz(tmp.Name(), target, 0); err != nil {
			return err
		}
	}

	if err := symlinkForce(target, filepath.Join(ctx.ToolsDir, "jdtls")); err != nil {
		return err
	}
	launcher := filepath.Join(target, "bin", "jdtls")
	if _, err := os.Stat(launcher); err != nil {
		launcher = filepath.Join(target, "jdtls")
	}
	return writeWrapper(filepath.Join(ctx.BinDir, "jdtls"), launcher)
}

// lombokMethod downloads the Lombok jar and points ToolsDir/lombok.jar at it.
type lombokMethod struct{ base }

func (lombokMethod) Install(t toolreg.Tool, ctx Context) error {
	versioned := filepath.Join(ctx.ToolsDir, "lombok-"+t.Version+".jar")
	if _, err := os.Stat(versioned); err != nil {
		f, err := os.Create(versioned)
		if err != nil {
			return err
		}
		url := fmt.Sprintf("https://projectlombok.org/downloads/lombok-%s.jar", t.Version)
		if err := download(url, f); err != nil {
			f.Close()
			_ = os.Remove(versioned)
			return err
		}
		f.Close()
	}
	return symlinkForce(versioned, filepath.Join(ctx.ToolsDir, "lombok.jar"))
}

// vsixMethod downloads a VS Code marketplace extension (a zip, possibly gzip'd)
// and unpacks it into ToolsDir/<name>-<version>, symlinked as ToolsDir/<name>.
type vsixMethod struct{ base }

func (vsixMethod) Install(t toolreg.Tool, ctx Context) error {
	target := filepath.Join(ctx.ToolsDir, t.Artifact+"-"+t.Version)
	if _, err := os.Stat(target); err != nil {
		url := strings.ReplaceAll(t.Pkg, "{version}", t.Version)
		tmp, err := os.CreateTemp("", t.Artifact+"-*.vsix")
		if err != nil {
			return err
		}
		defer cleanup(tmp.Name())
		if err := download(url, tmp); err != nil {
			return err
		}
		src, err := maybeGunzip(tmp.Name())
		if err != nil {
			return err
		}
		if err := os.MkdirAll(target, 0o755); err != nil {
			return err
		}
		if err := extractZip(src, target); err != nil {
			return err
		}
	}
	return symlinkForce(target, filepath.Join(ctx.ToolsDir, t.Artifact))
}

// ── low-level helpers ───────────────────────────────────────────────────────

func fetchText(url string) (string, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("could not download %s (%s) — check your network and retry", url, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	return string(b), err
}

func download(url string, dst io.Writer) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("could not download %s (%s) — check your network and retry", url, resp.Status)
	}
	_, err = io.Copy(dst, resp.Body)
	return err
}

// maybeGunzip returns a path to a decompressed copy when src is gzip'd (the
// marketplace sometimes returns gzip-encoded VSIX), otherwise src unchanged.
func maybeGunzip(src string) (string, error) {
	f, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer f.Close()
	magic := make([]byte, 2)
	if _, err := io.ReadFull(f, magic); err != nil {
		return src, nil //nolint:nilerr // too small to be gzip; use as-is
	}
	if magic[0] != 0x1f || magic[1] != 0x8b {
		return src, nil
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	out, err := os.CreateTemp("", "vsix-*.zip")
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, gz); err != nil { //nolint:gosec // trusted marketplace archive
		return "", err
	}
	return out.Name(), nil
}

// extractTarGz unpacks a .tar.gz into dir, dropping the first strip path
// components from each entry.
func extractTarGz(src, dir string, strip int) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := stripComponents(hdr.Name, strip)
		if name == "" {
			continue
		}
		dest, err := safeJoin(dir, name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := writeFileFrom(tr, dest, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		}
	}
}

// extractZip unpacks a zip archive into dir.
func extractZip(src, dir string) error {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, zf := range zr.File {
		dest, err := safeJoin(dir, zf.Name)
		if err != nil {
			return err
		}
		if zf.FileInfo().IsDir() {
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return err
			}
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		err = writeFileFrom(rc, dest, zf.Mode())
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// writeFileFrom writes r into path (creating parents) with the given mode. It
// bounds the copy to guard against decompression bombs from remote archives.
func writeFileFrom(r io.Reader, path string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode|0o200)
	if err != nil {
		return err
	}
	defer f.Close()
	const maxFileBytes = 512 << 20 // 512 MiB per file is ample for these tools
	if _, err := io.Copy(f, io.LimitReader(r, maxFileBytes)); err != nil {
		return err
	}
	return nil
}

// stripComponents removes the first n path components from a tar entry name.
func stripComponents(name string, n int) string {
	name = strings.TrimPrefix(filepath.Clean(name), "/")
	if n <= 0 {
		return name
	}
	parts := strings.Split(name, "/")
	if len(parts) <= n {
		return ""
	}
	return strings.Join(parts[n:], "/")
}

// safeJoin joins base and name, rejecting entries that would escape base (a
// zip/tar traversal guard).
func safeJoin(base, name string) (string, error) {
	dest := filepath.Join(base, name)
	if dest != base && !strings.HasPrefix(dest, base+string(os.PathSeparator)) {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	return dest, nil
}

// symlinkForce (re)creates link pointing at target.
func symlinkForce(target, link string) error {
	_ = os.Remove(link)
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return err
	}
	return os.Symlink(target, link)
}

// writeWrapper writes an executable shell wrapper at path that execs launcher.
func writeWrapper(path, launcher string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	script := fmt.Sprintf("#!/usr/bin/env bash\nexec %q \"$@\"\n", launcher)
	return os.WriteFile(path, []byte(script), 0o755) //nolint:gosec // intentional executable wrapper
}

func cleanup(path string) { _ = os.Remove(path) }
