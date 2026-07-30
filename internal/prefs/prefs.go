// Package prefs is karya's per-project preference store: a flat key=value file
// under the karya data prefix (config.Paths.PrefsFile). It is a faithful port of
// the load_pref/save_pref helpers in dotfiles/scripts/ide-agent.sh.
//
// The store keeps preferences that are not derivable from the code — chiefly the
// agent chosen for a given project directory (key "agent.<workdir>"). Like every
// piece of karya state it lives inside the karya prefix, never in the user's own
// config, preserving the isolation guarantee (see PLAN.md §2).
package prefs

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Store reads and writes a single flat key=value preferences file. The zero
// value is not usable; construct one with New.
type Store struct {
	path string
}

// New returns a Store backed by the file at path. The file and its parent
// directory are created lazily on the first write.
func New(path string) *Store {
	return &Store{path: path}
}

// Get returns the value for key, or "" if the key is absent (or the file does
// not exist). A value may itself contain '=' — only the first '=' delimits the
// key, matching `cut -d= -f2-`.
func (s *Store) Get(key string) string {
	for _, line := range s.lines() {
		if k, v, ok := strings.Cut(line, "="); ok && k == key {
			return v
		}
	}
	return ""
}

// Set stores value under key, replacing any existing entry for that key and
// preserving the order of the others. It creates the file (and parent dir) if
// needed.
func (s *Store) Set(key, value string) error {
	kept := s.without(key)
	kept = append(kept, key+"="+value)
	return s.write(kept)
}

// Delete removes any entry for key. It is a no-op if the key or file is absent.
func (s *Store) Delete(key string) error {
	if _, err := os.Stat(s.path); os.IsNotExist(err) {
		return nil
	}
	return s.write(s.without(key))
}

// Entries returns the stored preference lines ("key=value") in file order,
// skipping blanks. Used by `karya agent prefs` to display the store.
func (s *Store) Entries() []string {
	var out []string
	for _, line := range s.lines() {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

// Path returns the backing file path (for user-facing messages).
func (s *Store) Path() string { return s.path }

// lines returns the file's non-empty lines, or nil if the file is missing.
func (s *Store) lines() []string {
	f, err := os.Open(s.path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if line := sc.Text(); strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

// without returns all entries whose key differs from key, in order.
func (s *Store) without(key string) []string {
	var kept []string
	for _, line := range s.lines() {
		if k, _, ok := strings.Cut(line, "="); ok && k == key {
			continue
		}
		kept = append(kept, line)
	}
	return kept
}

// write atomically-enough replaces the file contents with the given lines.
func (s *Store) write(lines []string) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("prefs: create dir: %w", err)
	}
	var b strings.Builder
	for _, line := range lines {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(s.path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("prefs: write %s: %w", s.path, err)
	}
	return nil
}
