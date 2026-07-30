package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/drjzlyan/karya/internal/tutorial"
)

// cmdTutorial implements `karya tutorial [list|<lesson>]` — a self-working
// tutorial whose lessons run real karya behavior against a throwaway sandbox and
// verify it. With no argument it runs every lesson in order; with a number it
// runs just that lesson; `list` prints the lesson titles.
func cmdTutorial(args []string) int {
	if len(args) > 0 && args[0] == "list" {
		listLessons(os.Stdout)
		return 0
	}

	lessons, err := selectLessons(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "karya tutorial: %v\n", err)
		listLessons(os.Stderr)
		return 2
	}

	sb, err := tutorial.NewSandbox()
	if err != nil {
		return fail(err)
	}
	defer func() { _ = sb.Cleanup() }()

	// Pause between lessons only when a human is watching, so the flow stays
	// scriptable and testable when piped.
	interactive := isTerminal(os.Stdout) && isTerminal(os.Stdin)
	in := bufio.NewReader(os.Stdin)

	fmt.Fprintf(os.Stdout, "karya tutorial — running against a throwaway sandbox (%s)\n\n", sb.Dir)
	failures := 0
	for i, l := range lessons {
		if !tutorial.Render(os.Stdout, sb, l) {
			failures++
		}
		if interactive && i < len(lessons)-1 {
			fmt.Fprint(os.Stdout, "  Press Enter to continue… ")
			_, _ = in.ReadString('\n')
			fmt.Fprintln(os.Stdout)
		}
	}

	if failures > 0 {
		fmt.Fprintf(os.Stdout, "Finished with %d check(s) failing — see above, or run `karya doctor`.\n", failures)
		return 1
	}
	fmt.Fprintln(os.Stdout, "Tutorial complete. Try `karya install`, then `karya`.")
	return 0
}

// selectLessons resolves the tutorial args to the lessons to run: all of them
// when no argument is given, or the single lesson named by a 1-based number.
func selectLessons(args []string) ([]tutorial.Lesson, error) {
	all := tutorial.Lessons()
	if len(args) == 0 {
		return all, nil
	}
	n, err := strconv.Atoi(args[0])
	if err != nil {
		return nil, fmt.Errorf("lesson must be a number (1–%d) or `list`, got %q", len(all), args[0])
	}
	if n < 1 || n > len(all) {
		return nil, fmt.Errorf("no lesson %d (there are %d)", n, len(all))
	}
	return all[n-1 : n], nil
}

// listLessons prints the numbered lesson titles.
func listLessons(w io.Writer) {
	fmt.Fprintln(w, "Tutorial lessons (karya tutorial <n>):")
	for _, l := range tutorial.Lessons() {
		fmt.Fprintf(w, "  %d. %s\n", l.Num, l.Title)
	}
}
