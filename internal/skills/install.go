package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"path/filepath"
)

// Store is the on-disk set of installed skills under a root dir (the karya
// prefix's SkillsDir).
type Store struct{ Root string }

// Path returns where skill name is (or would be) installed.
func (s Store) Path(name string) string { return filepath.Join(s.Root, name) }

// Installed reports whether skill name is installed.
func (s Store) Installed(name string) bool {
	info, err := os.Stat(s.Path(name))
	return err == nil && info.IsDir()
}

// List returns the names of installed skills, sorted by directory order.
func (s Store) List() []string {
	entries, err := os.ReadDir(s.Root)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}

// Remove deletes an installed skill.
func (s Store) Remove(name string) error {
	if name == "" {
		return fmt.Errorf("skills: empty name")
	}
	return os.RemoveAll(s.Path(name))
}

// Install downloads every file of entry from src, verifies each against its
// pinned SHA-256, and lands the skill atomically in the store: it stages into a
// temp dir and only renames into place once every file verifies, so a checksum
// mismatch never leaves a half-installed (and unverified) skill.
func (s Store) Install(entry Entry, src Source) error {
	if entry.Name == "" {
		return fmt.Errorf("skills: entry has no name")
	}
	if len(entry.Files) == 0 {
		return fmt.Errorf("skills: %s has no files", entry.Name)
	}
	if err := os.MkdirAll(s.Root, 0o755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(s.Root, ".stage-"+entry.Name+"-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage) // no-op after a successful rename

	for _, f := range entry.Files {
		if err := safeRel(f.Path); err != nil {
			return err
		}
		data, err := src.Fetch(path.Join(entry.Name, f.Path))
		if err != nil {
			return fmt.Errorf("skills: fetch %s/%s: %w", entry.Name, f.Path, err)
		}
		if got := hashHex(data); got != f.SHA256 {
			return fmt.Errorf("skills: checksum mismatch for %s/%s: got %s want %s",
				entry.Name, f.Path, got, f.SHA256)
		}
		dest := filepath.Join(stage, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return err
		}
	}

	final := s.Path(entry.Name)
	if err := os.RemoveAll(final); err != nil {
		return err
	}
	return os.Rename(stage, final)
}

func hashHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
