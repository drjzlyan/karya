package skills

import (
	"os"
	"path/filepath"
	"strings"
)

// Materialization makes karya-installed skills visible to a coding agent by
// symlinking each installed skill into the agent's native skills directory
// (DESIGN.md §9). Symlinks (not copies) keep updates in sync and make removal
// clean: karya only ever manages links that point back into its own prefix, so
// it never disturbs a skill the user placed there themselves.

// Materialize symlinks every installed skill into targetDir. It is idempotent:
// existing karya-managed links are refreshed; foreign entries are left alone.
func (s Store) Materialize(targetDir string) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}
	for _, name := range s.List() {
		if err := s.link(targetDir, name); err != nil {
			return err
		}
	}
	return nil
}

// link creates or refreshes the symlink for one skill in targetDir.
func (s Store) link(targetDir, name string) error {
	dest := filepath.Join(targetDir, name)
	// Replace only a karya-managed link (or nothing); never clobber a real dir or
	// a foreign symlink the user owns.
	if info, err := os.Lstat(dest); err == nil {
		if info.Mode()&os.ModeSymlink == 0 || !s.owns(dest) {
			return nil // not ours — leave it
		}
		if err := os.Remove(dest); err != nil {
			return err
		}
	}
	return os.Symlink(s.Path(name), dest)
}

// Unlink removes the karya-managed symlink for one skill from targetDir.
func (s Store) Unlink(targetDir, name string) error {
	dest := filepath.Join(targetDir, name)
	if s.owns(dest) {
		return os.Remove(dest)
	}
	return nil
}

// Dematerialize removes every karya-managed skill symlink from targetDir,
// leaving anything the user owns untouched.
func (s Store) Dematerialize(targetDir string) error {
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return nil // nothing to clean
	}
	for _, e := range entries {
		dest := filepath.Join(targetDir, e.Name())
		if s.owns(dest) {
			if err := os.Remove(dest); err != nil {
				return err
			}
		}
	}
	return nil
}

// owns reports whether dest is a symlink pointing inside this store's root —
// i.e. a link karya created and may safely remove.
func (s Store) owns(dest string) bool {
	info, err := os.Lstat(dest)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return false
	}
	target, err := os.Readlink(dest)
	if err != nil {
		return false
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(dest), target)
	}
	root, _ := filepath.Abs(s.Root)
	target, _ = filepath.Abs(target)
	return strings.HasPrefix(target, root+string(os.PathSeparator))
}
