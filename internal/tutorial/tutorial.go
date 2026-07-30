// Package tutorial is karya's self-guided, self-working tutorial. Unlike the
// narrative docs (docs/tutorial.md, surfaced by `karya docs tutorial`), the
// lessons here actually execute real karya behavior against a throwaway sandbox
// and verify the result — so the tutorial proves the tool works on the user's
// machine rather than merely describing it.
//
// The package is deliberately pure and side-effect-scoped: every mutation lands
// inside a Sandbox temp directory, and rendering writes to an injected
// io.Writer, so the whole flow is exercised by ordinary unit tests. Interactive
// pacing (pause between lessons) lives in the CLI layer, not here.
package tutorial

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/drjzlyan/karya/internal/agent"
	"github.com/drjzlyan/karya/internal/assets"
	"github.com/drjzlyan/karya/internal/config"
	"github.com/drjzlyan/karya/internal/project"
	"github.com/drjzlyan/karya/internal/tmuxx"
)

// Sandbox is a throwaway workspace a lesson may write into. It is created fresh
// per tutorial run and removed on Cleanup, so lessons never touch real projects.
type Sandbox struct {
	Dir string
}

// NewSandbox creates an isolated temporary directory for the tutorial to work in.
func NewSandbox() (*Sandbox, error) {
	dir, err := os.MkdirTemp("", "karya-tutorial-*")
	if err != nil {
		return nil, fmt.Errorf("create tutorial sandbox: %w", err)
	}
	return &Sandbox{Dir: dir}, nil
}

// Cleanup removes the sandbox directory. It is safe to call more than once.
func (s *Sandbox) Cleanup() error {
	if s == nil || s.Dir == "" {
		return nil
	}
	return os.RemoveAll(s.Dir)
}

// Lesson is one numbered step of the tutorial. Body is the explanation; Run,
// when non-nil, makes the lesson self-working — it performs real karya behavior
// against the sandbox and returns a one-line detail of what it verified, or an
// error if the check failed. A lesson with a nil Run is purely explanatory.
type Lesson struct {
	Num   int
	Title string
	Body  string
	Run   func(sb *Sandbox) (detail string, err error)
}

// Lessons returns the ordered tutorial. Numbering is assigned from position so
// it can never drift from the slice order.
func Lessons() []Lesson {
	ls := []Lesson{
		{
			Title: "Everything karya touches is isolated",
			Body: "karya never reads or writes your own Neovim, tmux, or shell config.\n" +
				"All of its state lives under a karya-namespaced prefix, so you can try\n" +
				"it with zero risk to your existing setup.",
			Run: verifyIsolation,
		},
		{
			Title: "Scaffold a project",
			Body: "`karya new <lang> <name>` generates a ready-to-run project and opens it\n" +
				"in an IDE session. Languages: " + strings.Join(project.Languages, ", ") + ".",
			Run: verifyScaffold,
		},
		{
			Title: "Projects start as git repositories",
			Body: "Every scaffolded project is initialized as a git repo, so you can commit\n" +
				"immediately (press Ctrl-a g in a session to open lazygit).",
			Run: verifyGitInit,
		},
		{
			Title: "The docs travel inside the binary",
			Body: "`karya docs <topic>` and `karya help <command>` work fully offline — the\n" +
				"documentation is embedded in the binary, so no browser or repo is needed.",
			Run: verifyEmbeddedDocs,
		},
		{
			Title: "Your IDE session",
			Body: "Run `karya` in any project to launch a tmux session with three panes —\n" +
				"editor (Neovim), coding agent, and build/test — plus a git window.\n" +
				"The tmux prefix is Ctrl-a; press Ctrl-a ? in a session for the key map.",
			Run: verifyTmux,
		},
		{
			Title: "Coding agents",
			Body: "karya detects installed AI coding agents and wires one into the session.\n" +
				"Switch with Ctrl-a A (or cycle with Ctrl-a N); your choice is remembered\n" +
				"per project. Use `karya dev -a none` for a plain shell.",
			Run: verifyAgents,
		},
		{
			Title: "Where to go next",
			Body: "That's the core loop. Read the full walkthrough with `karya docs tutorial`,\n" +
				"the complete key map with `karya docs keymaps`, and pick languages with\n" +
				"`karya lang`. When you're ready: `karya install`, then `karya`.",
		},
	}
	for i := range ls {
		ls[i].Num = i + 1
	}
	return ls
}

// Render writes one lesson to w and, when the lesson is self-working, executes
// its check and reports the outcome. It returns false only when a verification
// step ran and failed, so callers can surface an environment problem.
func Render(w io.Writer, sb *Sandbox, l Lesson) bool {
	total := len(Lessons())
	header := fmt.Sprintf("[%d/%d] %s", l.Num, total, l.Title)
	fmt.Fprintf(w, "%s\n%s\n", header, strings.Repeat("─", len(header)))
	fmt.Fprintf(w, "%s\n", l.Body)
	if l.Run == nil {
		fmt.Fprintln(w)
		return true
	}
	detail, err := l.Run(sb)
	if err != nil {
		fmt.Fprintf(w, "\n  ✗ %v\n\n", err)
		return false
	}
	fmt.Fprintf(w, "\n  ✓ %s\n\n", detail)
	return true
}

// verifyIsolation checks that every karya path is namespaced under the karya
// prefix and separate from the user's own config, demonstrating the isolation
// guarantee without touching anything.
func verifyIsolation(*Sandbox) (string, error) {
	p := config.Resolve()
	roots := map[string]string{
		"Config": p.Config, "Data": p.Data, "State": p.State, "Cache": p.Cache,
	}
	for name, dir := range roots {
		if filepath.Base(dir) != config.AppName {
			return "", fmt.Errorf("%s dir %q is not namespaced under %q", name, dir, config.AppName)
		}
	}
	// The editor config nests under karya, never the user's ~/.config/nvim.
	if want := filepath.Join(p.Config, "nvim"); p.NvimConfig() != want {
		return "", fmt.Errorf("NvimConfig %q is not inside the karya prefix", p.NvimConfig())
	}
	return fmt.Sprintf("all state under a %q prefix (e.g. %s)", config.AppName, p.Config), nil
}

// verifyScaffold generates a real project in the sandbox and confirms its files
// exist, exercising the same code path as `karya new`.
func verifyScaffold(sb *Sandbox) (string, error) {
	spec, err := project.NewSpec("go", "example.com/hello")
	if err != nil {
		return "", err
	}
	dir, err := project.Scaffold(filepath.Join(sb.Dir, "scaffold"), spec)
	if err != nil {
		return "", err
	}
	want := []string{"go.mod", filepath.Join("cmd", spec.Basename, "main.go"), ".gitignore"}
	for _, f := range want {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			return "", fmt.Errorf("expected %s in scaffolded project: %w", f, err)
		}
	}
	return fmt.Sprintf("created a go project (go.mod, cmd/%s/main.go, .gitignore) at %s", spec.Basename, dir), nil
}

// verifyGitInit scaffolds a project and initializes it as a git repo, verifying
// the .git directory appears. It degrades gracefully when git is not installed.
func verifyGitInit(sb *Sandbox) (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "skipped — git is not installed (install git to use this)", nil
	}
	spec, err := project.NewSpec("python", "gitdemo")
	if err != nil {
		return "", err
	}
	dir, err := project.Scaffold(filepath.Join(sb.Dir, "git"), spec)
	if err != nil {
		return "", err
	}
	if err := project.GitInit(dir); err != nil {
		return "", err
	}
	if st, err := os.Stat(filepath.Join(dir, ".git")); err != nil || !st.IsDir() {
		return "", fmt.Errorf("expected a .git directory in %s", dir)
	}
	return "initialized a git repository (.git created)", nil
}

// verifyEmbeddedDocs confirms the offline documentation is present in the binary.
func verifyEmbeddedDocs(*Sandbox) (string, error) {
	topics := assets.DocTopics()
	if len(topics) == 0 {
		return "", fmt.Errorf("no documentation is embedded in this build")
	}
	if _, ok := assets.Doc("tutorial"); !ok {
		return "", fmt.Errorf("the tutorial doc is not embedded")
	}
	return fmt.Sprintf("embedded topics available offline: %s", strings.Join(topics, ", ")), nil
}

// verifyTmux reports whether tmux — required for the IDE session — is available.
func verifyTmux(*Sandbox) (string, error) {
	if !tmuxx.Available() {
		return "", fmt.Errorf("tmux is not installed; install it, then run `karya doctor`")
	}
	return "tmux is installed — `karya` can launch the IDE session", nil
}

// verifyAgents reports which coding agents are detected on this machine.
func verifyAgents(*Sandbox) (string, error) {
	detected := agent.Detect()
	if len(detected) == 0 {
		return fmt.Sprintf("no agents detected yet — install any of: %s",
			strings.Join(agent.Known, ", ")), nil
	}
	return "detected agents: " + strings.Join(detected, ", "), nil
}
