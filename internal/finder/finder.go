// Package finder is karya's fuzzy file finder — the human's way to jump to any
// file in the repo without leaving the IDE (DESIGN.md §6.4). It lists the repo's
// files (ripgrep when available, a filesystem walk otherwise), fuzzy-filters as
// you type, and opens the selection in the editor pane. The listing and the
// fuzzy match are pure and unit-tested; the view is a thin karya paneView.
package finder

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// skipDirs are directories never worth listing.
var skipDirs = map[string]bool{
	".git": true, "node_modules": true, ".karya": true,
	"vendor": true, "target": true, "dist": true, "build": true, ".venv": true,
}

// ListFiles returns repo-relative file paths under root, preferring `rg --files`
// (fast, respects .gitignore) and falling back to a filesystem walk.
func ListFiles(root string) []string {
	if files, ok := ripgrepFiles(root); ok {
		return files
	}
	return walkFiles(root)
}

func ripgrepFiles(root string) ([]string, bool) {
	if _, err := exec.LookPath("rg"); err != nil {
		return nil, false
	}
	cmd := exec.Command("rg", "--files", "--hidden", "--glob", "!.git")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	var files []string
	for _, l := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if l != "" {
			files = append(files, l)
		}
	}
	return files, true
}

func walkFiles(root string) []string {
	var files []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if rel, err := filepath.Rel(root, path); err == nil {
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	return files
}

// Match reports whether query is a fuzzy (subsequence) match of cand and, if so,
// a score: higher is better. Consecutive matches and matches at a path segment
// boundary score more; shorter candidates are preferred. An empty query matches
// everything with score 0.
func Match(query, cand string) (int, bool) {
	if query == "" {
		return 0, true
	}
	q := strings.ToLower(query)
	c := strings.ToLower(cand)
	score, qi, prev := 0, 0, -2
	for ci := 0; ci < len(c) && qi < len(q); ci++ {
		if c[ci] != q[qi] {
			continue
		}
		if ci == prev+1 {
			score += 5 // consecutive
		}
		if ci == 0 || c[ci-1] == '/' || c[ci-1] == '_' || c[ci-1] == '-' || c[ci-1] == '.' {
			score += 10 // start of a path/word segment
		}
		prev = ci
		qi++
	}
	if qi != len(q) {
		return 0, false
	}
	score -= len(c) - len(q) // prefer shorter paths
	return score, true
}

// Filter returns the items matching query, best score first (ties broken by
// path). An empty query returns items unchanged.
func Filter(query string, items []string) []string {
	if strings.TrimSpace(query) == "" {
		return items
	}
	type scored struct {
		item  string
		score int
	}
	var matched []scored
	for _, it := range items {
		if s, ok := Match(query, it); ok {
			matched = append(matched, scored{it, s})
		}
	}
	sort.SliceStable(matched, func(i, j int) bool {
		if matched[i].score != matched[j].score {
			return matched[i].score > matched[j].score
		}
		return matched[i].item < matched[j].item
	})
	out := make([]string, len(matched))
	for i, m := range matched {
		out[i] = m.item
	}
	return out
}
