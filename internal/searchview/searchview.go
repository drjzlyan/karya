// Package searchview is karya's project-wide search — the human's grep across
// the repo without leaving the IDE (DESIGN.md §6.4). It runs ripgrep for a
// query, lists file:line matches, and opens the chosen one at its location in
// the editor. The ripgrep call is behind a Searcher func so the view is
// unit-testable with a fake.
package searchview

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Match is one search hit.
type Match struct {
	File string
	Line int
	Text string
}

// Searcher runs a search in dir and returns matches (empty query → no matches).
type Searcher func(dir, query string) []Match

// maxMatches caps results so a broad query stays responsive.
const maxMatches = 500

// Ripgrep is the production Searcher: `rg` line-numbered matches, capped and
// time-bounded so it never hangs the UI.
func Ripgrep(dir, query string) []Match {
	if strings.TrimSpace(query) == "" {
		return nil
	}
	if _, err := exec.LookPath("rg"); err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "rg", "--no-heading", "--line-number",
		"--color=never", "--max-columns=300", "--", query)
	cmd.Dir = dir
	out, _ := cmd.Output() // rg exits non-zero on no matches; parse whatever came
	return parse(string(out))
}

// parse turns `path:line:text` ripgrep output into matches.
func parse(out string) []Match {
	var matches []Match
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}
		n, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}
		matches = append(matches, Match{File: parts[0], Line: n, Text: parts[2]})
		if len(matches) >= maxMatches {
			break
		}
	}
	return matches
}
