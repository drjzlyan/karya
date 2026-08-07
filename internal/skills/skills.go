// Package skills is karya's skills marketplace client: it reads a versioned
// registry index, installs skill packages (a SKILL.md plus assets) into the
// karya prefix with content-hash verification, and tracks what is installed
// (DESIGN.md §9). Skills are prompt-bearing code, so every file is checksum-
// verified before it lands; nothing is written outside karya-owned dirs.
//
// Network access is behind the Source interface, so the registry client and the
// installer are fully unit-testable with an in-memory or on-disk source.
package skills

import (
	"strings"
)

// File is one file in a skill package, pinned to its content hash.
type File struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// Entry is one skill in the registry index.
type Entry struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	License     string   `json:"license,omitempty"`
	Files       []File   `json:"files"`
	MinCaps     []string `json:"min_caps,omitempty"` // required agent capabilities
}

// Index is the registry's catalog of installable skills.
type Index struct {
	Skills []Entry `json:"skills"`
}

// Find returns the entry with the given name, or ok=false.
func (i *Index) Find(name string) (Entry, bool) {
	for _, e := range i.Skills {
		if e.Name == name {
			return e, true
		}
	}
	return Entry{}, false
}

// Search returns entries whose name or description contains query (case-
// insensitive). An empty query returns all entries.
func (i *Index) Search(query string) []Entry {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return i.Skills
	}
	var out []Entry
	for _, e := range i.Skills {
		if strings.Contains(strings.ToLower(e.Name), q) ||
			strings.Contains(strings.ToLower(e.Description), q) {
			out = append(out, e)
		}
	}
	return out
}
