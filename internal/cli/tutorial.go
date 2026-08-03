package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/drjzlyan/karya/internal/agent"
	"github.com/drjzlyan/karya/internal/project"
	"github.com/drjzlyan/karya/internal/session"
	"github.com/drjzlyan/karya/internal/tutorial"
)

// cmdTutorial implements `karya tutorial [list|ide|<lesson>]` — a hands-on
// walkthrough. Each lesson asks you to type a karya command yourself, then runs
// the real behavior against a throwaway sandbox and verifies it. With no argument
// it walks every lesson in order; a number runs just that one; `list` prints the
// titles; `ide` launches the keystroke-driven in-editor tutorial.
func cmdTutorial(args []string) int {
	if len(args) > 0 {
		switch args[0] {
		case "list":
			listLessons(os.Stdout)
			return 0
		case "ide":
			return cmdTutorialIDE(args[1:])
		}
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

	// Prompt for typed commands only when a human is driving. Piped/redirected
	// runs stay scriptable: they auto-render each lesson without waiting on input,
	// so CI and `karya tutorial | cat` behave exactly as before.
	interactive := isTerminal(os.Stdout) && isTerminal(os.Stdin)
	return runTutorial(os.Stdin, os.Stdout, interactive, sb, lessons)
}

// runTutorial walks the lessons, writing to out and (when interactive) reading
// typed commands from in. It returns a process exit code: non-zero when a
// verification failed. Extracted from cmdTutorial so the typing loop is testable
// with injected readers/writers.
func runTutorial(in io.Reader, out io.Writer, interactive bool, sb *tutorial.Sandbox, lessons []tutorial.Lesson) int {
	r := bufio.NewReader(in)

	fmt.Fprintf(out, "karya tutorial — you type the commands; a throwaway sandbox verifies them (%s)\n\n", sb.Dir)
	failures := 0
	for i, l := range lessons {
		if interactive && l.Expect != "" {
			promptForCommand(r, out, l)
		}
		if !tutorial.Render(out, sb, l) {
			failures++
		}
		if interactive && i < len(lessons)-1 {
			fmt.Fprint(out, "  Press Enter to continue… ")
			_, _ = r.ReadString('\n')
			fmt.Fprintln(out)
		}
	}

	if failures > 0 {
		fmt.Fprintf(out, "Finished with %d check(s) failing — see above, or run `karya doctor`.\n", failures)
		return 1
	}
	fmt.Fprintln(out, "Tutorial complete. Try `karya tutorial ide`, then `karya install` and `karya`.")
	return 0
}

// promptForCommand asks the learner to type the lesson's expected command,
// re-prompting on a mismatch (with the lesson's hint) until it matches or they
// type `skip`. It reads from r and writes prompts to out.
func promptForCommand(r *bufio.Reader, out io.Writer, l tutorial.Lesson) {
	fmt.Fprintf(out, "  Now try it — type:  %s\n", l.Expect)
	for {
		fmt.Fprint(out, "  › ")
		typed, err := r.ReadString('\n')
		if err != nil && typed == "" {
			return // EOF: stop prompting, fall through to the verification.
		}
		if tutorial.MatchCommand(l.Expect, typed) {
			return
		}
		if tutorial.MatchCommand("skip", typed) {
			fmt.Fprintln(out, "  (skipped)")
			return
		}
		hint := l.Hint
		if hint == "" {
			hint = "type it exactly, or `skip` to move on"
		}
		fmt.Fprintf(out, "  Not quite — %s\n", hint)
	}
}

// cmdTutorialIDE launches the in-editor, keystroke-driven tutorial. It scaffolds
// a throwaway sandbox project, opens a karya IDE session on it, and fires
// `:KaryaTutorial` in the editor pane so nvim, tmux, lazygit and the agent bridge
// can all be practiced against real (disposable) code. When tmux is unavailable
// it prints how to start the tutorial from inside a session instead.
func cmdTutorialIDE(_ []string) int {
	a, err := newApp()
	if err != nil {
		return fail(err)
	}

	sb, err := tutorial.NewSandbox()
	if err != nil {
		return fail(err)
	}
	// The sandbox is intentionally left in place: the IDE session lives on past
	// this process, so cleaning up here would delete the project out from under it.

	spec, err := project.NewSpec("go", "example.com/karya-tutorial")
	if err != nil {
		return fail(err)
	}
	dir, err := project.Scaffold(filepath.Join(sb.Dir, "tutorial"), spec)
	if err != nil {
		return fail(err)
	}
	if err := project.GitInit(dir); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}

	if err := ensureRuntime(a); err != nil {
		return fail(err)
	}

	// Pick the tutorial language up front (the user's primary selected language,
	// else Go) so the in-editor tutorial starts directly instead of falling back
	// to Neovim's default vim.ui.select picker, which renders as a messy
	// command-line prompt during startup. Invoke the engine's Lua function
	// directly rather than the :KaryaTutorial command: passing the language as a
	// command argument (`:KaryaTutorial python`) hard-errors with E488 against an
	// older extracted config whose command takes no args, whereas a stale Lua
	// function simply ignores the extra argument and falls back to the picker.
	tutLang := "go"
	if sel, err := loadSelection(a); err == nil && !sel.Empty() {
		if langs := sel.Langs(); len(langs) > 0 {
			tutLang = langs[0]
		}
	}

	name := "karya-tutorial"
	detected := agent.Detect()
	if err := session.Dev(a.tmux, session.Options{
		Name:     name,
		Workdir:  dir,
		Agent:    agent.Resolve("", detected),
		Detected: detected,
		Kill:     true,
		NvimInit: fmt.Sprintf(`lua require("tutorial.engine").start(%q)`, tutLang),
	}); err != nil {
		fmt.Fprintln(os.Stderr, "karya tutorial ide: could not launch a session.")
		fmt.Fprintln(os.Stderr, "Start karya in any project, then run :KaryaTutorial inside the editor.")
		return fail(err)
	}
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
		return nil, fmt.Errorf("lesson must be a number (1–%d), `list`, or `ide`, got %q", len(all), args[0])
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
	fmt.Fprintln(w, "Or: karya tutorial ide   (keystroke-driven, inside the IDE)")
}
