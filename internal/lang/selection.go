package lang

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Selection is an ordered set of languages and their chosen runtime versions.
// The first version of each language is the primary (default); the rest are
// installed side-by-side. Order is preserved so the file stays stable across
// edits.
type Selection struct {
	order    []string            // language names in insertion order
	versions map[string][]string // language name → versions (primary first)
}

// NewSelection returns an empty Selection.
func NewSelection() *Selection {
	return &Selection{versions: map[string][]string{}}
}

// Set records versions for a language, replacing any existing entry and keeping
// the language's original position when it was already present. Empty or
// whitespace-only versions are dropped; setting an empty list removes the
// language.
func (s *Selection) Set(lang string, versions []string) {
	cleaned := cleanVersions(versions)
	if len(cleaned) == 0 {
		s.Remove(lang)
		return
	}
	if _, ok := s.versions[lang]; !ok {
		s.order = append(s.order, lang)
	}
	s.versions[lang] = cleaned
}

// Remove deletes a language from the selection. It is a no-op if absent.
func (s *Selection) Remove(lang string) {
	if _, ok := s.versions[lang]; !ok {
		return
	}
	delete(s.versions, lang)
	for i, l := range s.order {
		if l == lang {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}
}

// Clear removes every selected language, leaving an empty selection.
func (s *Selection) Clear() {
	s.order = nil
	s.versions = map[string][]string{}
}

// Has reports whether a language is selected.
func (s *Selection) Has(lang string) bool { _, ok := s.versions[lang]; return ok }

// Versions returns the versions for a language (primary first), or nil.
func (s *Selection) Versions(lang string) []string { return s.versions[lang] }

// Primary returns the default version for a language, or "".
func (s *Selection) Primary(lang string) string {
	if v := s.versions[lang]; len(v) > 0 {
		return v[0]
	}
	return ""
}

// Langs returns the selected language names in order.
func (s *Selection) Langs() []string {
	out := make([]string, len(s.order))
	copy(out, s.order)
	return out
}

// Empty reports whether nothing is selected.
func (s *Selection) Empty() bool { return len(s.order) == 0 }

// cleanVersions trims whitespace and drops empty entries, preserving order.
func cleanVersions(versions []string) []string {
	out := make([]string, 0, len(versions))
	for _, v := range versions {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// ParseSelection reads a languages.local body: one `lang=v1,v2` per line,
// comment (#) and blank lines ignored. Later duplicate keys replace earlier
// ones. Only the first '=' delimits the key.
func ParseSelection(data string) *Selection {
	s := NewSelection()
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		s.Set(key, strings.Split(val, ","))
	}
	return s
}

// Render serialises the selection to languages.local format with a header. The
// output is deterministic for a given selection.
func (s *Selection) Render() string {
	var b strings.Builder
	b.WriteString("# Languages and runtime versions configured for karya.\n")
	b.WriteString("# Format: language=version[,version,...] — first version is primary.\n")
	b.WriteString("# Edit manually or re-run `karya lang`.\n\n")
	for _, lang := range s.order {
		b.WriteString(lang)
		b.WriteByte('=')
		b.WriteString(strings.Join(s.versions[lang], ","))
		b.WriteByte('\n')
	}
	return b.String()
}

// LoadSelection reads the selection from path. A missing file yields an empty
// selection and no error.
func LoadSelection(path string) (*Selection, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewSelection(), nil
		}
		return nil, err
	}
	defer f.Close()

	var b strings.Builder
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		b.WriteString(sc.Text())
		b.WriteByte('\n')
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return ParseSelection(b.String()), nil
}

// SaveSelection writes the selection to path, creating the parent dir.
func SaveSelection(path string, s *Selection) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("lang: create dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(s.Render()), 0o644); err != nil {
		return fmt.Errorf("lang: write %s: %w", path, err)
	}
	return nil
}
