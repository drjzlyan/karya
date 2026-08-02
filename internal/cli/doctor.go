package cli

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/drjzlyan/karya/internal/config"
	"github.com/drjzlyan/karya/internal/doctor"
	"github.com/drjzlyan/karya/internal/lang"
	"github.com/drjzlyan/karya/internal/version"
)

// cmdDoctor runs karya's health checks and prints a grouped report. It exits
// non-zero when any check is at Problem level, so scripts and `karya tutorial`
// can gate on a healthy environment. With --check-updates it also queries mise
// for available tool updates (network).
func cmdDoctor(args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	checkUpdates := fs.Bool("check-updates", false, "check managed tools for available updates (network)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	p := config.Resolve()
	// See and run the tools karya installed into its isolated prefix (tmux,
	// Neovim, mise, …) so the report reflects karya's managed runtime, including
	// version probes, not just the user's PATH.
	p.ActivateManagedEnv()
	sel, err := lang.LoadSelection(p.LanguagesFile())
	if err != nil {
		// A malformed selection file should not stop the rest of the diagnosis.
		fmt.Fprintf(os.Stderr, "warning: could not read language selection: %v\n", err)
		sel = lang.NewSelection()
	}

	report := doctor.Run(doctor.Probe{
		Paths:        p,
		Selection:    sel,
		KaryaVersion: version.String(),
		CheckUpdates: *checkUpdates,
	})
	renderReport(os.Stdout, report)

	if !report.Healthy() {
		return 1
	}
	return 0
}

// renderReport writes the report grouped by section, with a per-check marker and
// a final summary line.
func renderReport(w io.Writer, r doctor.Report) {
	fmt.Fprintln(w, "karya doctor")
	group := ""
	for _, c := range r.Checks {
		if c.Group != group {
			group = c.Group
			fmt.Fprintf(w, "\n%s\n", group)
		}
		fmt.Fprintf(w, "  %s %-16s %s\n", marker(c.Level), c.Name, c.Detail)
	}
	fmt.Fprintf(w, "\n%d ok · %d warning(s) · %d problem(s)\n",
		r.Count(doctor.OK), r.Count(doctor.Warn), r.Count(doctor.Problem))
	if r.Healthy() {
		fmt.Fprintln(w, "All required checks passed.")
	} else {
		fmt.Fprintln(w, "Some required checks failed — see the ✗ items above.")
	}
}

// marker returns the status glyph for a check level.
func marker(level doctor.Level) string {
	switch level {
	case doctor.OK:
		return "✓"
	case doctor.Warn:
		return "•"
	default:
		return "✗"
	}
}
