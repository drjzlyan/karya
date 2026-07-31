// Package tools installs the runtimes, language servers, formatters, linters,
// debuggers, and CLI utilities karya manages — into karya's own isolated tool
// prefix, never into Homebrew or the user's global environment (PLAN.md §2, §6.4).
//
// What to install and in what order is decided by the pure internal/toolreg
// registry; this package is the detect-first, best-effort side-effect layer: a
// tool already resolvable is left alone, and anything karya installs lands under
// config.Paths so uninstall is a single directory removal.
package tools

import (
	"fmt"
	"strings"
)

// Status is the outcome of attempting one tool.
type Status int

const (
	// Installed means karya installed the tool this run.
	Installed Status = iota
	// Skipped means the tool was already available.
	Skipped
	// Missing means a detect-only tool is absent (a hint was printed).
	Missing
	// Failed means installation was attempted and errored.
	Failed
)

// Result pairs a tool with its outcome.
type Result struct {
	Tool   string
	Status Status
	Err    error
}

// Summarize renders a one-line count of results for the CLI.
func Summarize(results []Result) string {
	var installed, skipped, missing, failed int
	for _, r := range results {
		switch r.Status {
		case Installed:
			installed++
		case Skipped:
			skipped++
		case Missing:
			missing++
		case Failed:
			failed++
		}
	}
	parts := []string{
		fmt.Sprintf("%d installed", installed),
		fmt.Sprintf("%d already present", skipped),
	}
	if missing > 0 {
		parts = append(parts, fmt.Sprintf("%d need manual install", missing))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}
	return strings.Join(parts, ", ")
}
