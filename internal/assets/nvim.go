package assets

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// nvimFS holds the vendored Neovim configuration (init.lua, lua/**, the plugin
// lockfile and the after/ override dir). `all:` includes hidden files such as
// after/.gitkeep. This tree is the source of truth; edit it directly.
//
//go:embed all:nvim
var nvimFS embed.FS

// nvimRoot is the embed prefix stripped when extracting so the config lands at
// the destination root (mirroring a standard ~/.config/nvim layout).
const nvimRoot = "nvim"

// manifestName is the version record written alongside the extracted config so
// `karya update` re-extracts only when the embedded config actually changed.
const manifestName = "manifest.json"

// manifest is the on-disk version record for the extracted Neovim config.
type manifest struct {
	Version string `json:"version"`
}

// NvimVersion returns a stable content hash of the embedded Neovim config. It is
// derived from every embedded file's path and bytes in sorted order, so it
// changes precisely when the vendored config changes and is identical across
// builds and machines.
func NvimVersion() (string, error) {
	h := sha256.New()
	err := fs.WalkDir(nvimFS, nvimRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := nvimFS.ReadFile(path)
		if err != nil {
			return err
		}
		// Hash the path then the bytes so renames and edits both change the digest.
		fmt.Fprintf(h, "%s\n%d\n", path, len(data))
		h.Write(data)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("hash embedded nvim config: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ExtractNvimConfig writes the embedded Neovim config tree to destDir and
// records its version in a manifest. It replaces destDir wholesale so files
// removed upstream do not linger; the extracted config is karya-owned and holds
// nothing else (plugins and state live under the isolated data/state dirs).
func ExtractNvimConfig(destDir string) error {
	version, err := NvimVersion()
	if err != nil {
		return err
	}
	if err := os.RemoveAll(destDir); err != nil {
		return fmt.Errorf("clean nvim config dir: %w", err)
	}
	err = fs.WalkDir(nvimFS, nvimRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(nvimRoot, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destDir, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := nvimFS.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		return fmt.Errorf("extract nvim config: %w", err)
	}
	return writeManifest(destDir, version)
}

// EnsureNvimConfig extracts the embedded Neovim config to destDir only when it is
// missing or its recorded version differs from the embedded one. It reports
// whether it (re-)extracted, which callers use to decide when to sync plugins.
func EnsureNvimConfig(destDir string) (bool, error) {
	version, err := NvimVersion()
	if err != nil {
		return false, err
	}
	if readManifest(destDir) == version {
		return false, nil
	}
	if err := ExtractNvimConfig(destDir); err != nil {
		return false, err
	}
	return true, nil
}

// writeManifest records version in destDir's manifest file.
func writeManifest(destDir, version string) error {
	data, err := json.Marshal(manifest{Version: version})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(destDir, manifestName), data, 0o644)
}

// readManifest returns the version recorded at destDir, or "" if absent/unreadable.
func readManifest(destDir string) string {
	raw, err := os.ReadFile(filepath.Join(destDir, manifestName))
	if err != nil {
		return ""
	}
	var m manifest
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	return m.Version
}
