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
	"github.com/drjzlyan/karya/internal/ship"
	"github.com/drjzlyan/karya/internal/spec"
	"github.com/drjzlyan/karya/internal/task"
	"github.com/drjzlyan/karya/internal/tmuxx"
	"github.com/drjzlyan/karya/internal/worktree"
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

// Outcome is the result of a self-working lesson's check.
type Outcome int

const (
	// Pass — a verification succeeded (✓).
	Pass Outcome = iota
	// Note — an informational environment report, not a failure (•). Used when
	// an optional tool (tmux, a coding agent) is not installed yet: the tutorial
	// should guide, not fail, on a fresh machine mid-setup. `karya doctor` is the
	// strict checker.
	Note
	// Fail — a real problem: karya behavior did not work as expected (✗).
	Fail
)

// Lesson is one numbered step of the tutorial. Body is the explanation; Run,
// when non-nil, verifies real karya behavior against the sandbox and returns its
// Outcome plus a one-line detail. A lesson with a nil Run is purely explanatory.
//
// Expect, when non-empty, is the command the learner is asked to type before the
// lesson proceeds — the tutorial is a walkthrough, so the user drives it. Hint is
// shown when they mistype. The typed command is matched (via MatchCommand) to
// gate progress; the actual proof of behavior is Run against the sandbox, so the
// tutorial demonstrates the tool works on the user's machine rather than merely
// echoing what they typed.
type Lesson struct {
	Num    int
	Title  string
	Body   string
	Expect string
	Hint   string
	Run    func(sb *Sandbox) (Outcome, string)
}

// MatchCommand reports whether a learner's typed command matches the expected
// one. It is lenient about surrounding and repeated whitespace (so trailing
// spaces or double spaces are fine) but case-sensitive on the command itself.
func MatchCommand(expected, typed string) bool {
	return normalizeCommand(expected) == normalizeCommand(typed)
}

// normalizeCommand trims and collapses internal whitespace so two commands that
// differ only in spacing compare equal.
func normalizeCommand(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// Lessons returns the ordered tutorial. Numbering is assigned from position so
// it can never drift from the slice order.
func Lessons() []Lesson {
	ls := []Lesson{
		{
			Title: "Everything karya touches is isolated",
			Body: "karya never reads or writes your own Neovim, tmux, or shell config.\n" +
				"All of its state lives under a karya-namespaced prefix, so you can try\n" +
				"it with zero risk to your existing setup. `karya doctor` reports the\n" +
				"isolated paths and tool status.",
			Expect: "karya doctor",
			Hint:   "type it exactly: karya doctor",
			Run:    verifyIsolation,
		},
		{
			Title: "Scaffold a project",
			Body: "`karya new <lang> <name>` generates a ready-to-run project and opens it\n" +
				"in an IDE session. Languages: " + strings.Join(project.Languages, ", ") + ".",
			Expect: "karya new go example.com/hello",
			Hint:   "type it exactly: karya new go example.com/hello",
			Run:    verifyScaffold,
		},
		{
			Title: "Projects start as git repositories",
			Body: "Every scaffolded project is initialized as a git repo, so you can commit\n" +
				"immediately (press Ctrl-a g in a session to open lazygit). Scaffold one now\n" +
				"and karya will confirm the .git directory was created.",
			Expect: "karya new python gitdemo",
			Hint:   "type it exactly: karya new python gitdemo",
			Run:    verifyGitInit,
		},
		{
			Title: "The docs travel inside the binary",
			Body: "`karya docs <topic>` and `karya help <command>` work fully offline — the\n" +
				"documentation is embedded in the binary, so no browser or repo is needed.\n" +
				"Ask for the tutorial topic to see it come straight from the binary.",
			Expect: "karya docs tutorial",
			Hint:   "type it exactly: karya docs tutorial",
			Run:    verifyEmbeddedDocs,
		},
		{
			Title: "Your IDE session",
			Body: "Run `karya` in any project to launch a tmux session with three panes —\n" +
				"editor (Neovim), coding agent, and build/test — plus a git window.\n" +
				"The tmux prefix is Ctrl-a; press Ctrl-a ? in a session for the key map,\n" +
				"or browse the whole reference now with `karya keys`.",
			Expect: "karya keys",
			Hint:   "type it exactly: karya keys",
			Run:    verifyTmux,
		},
		{
			Title: "Coding agents",
			Body: "karya detects installed AI coding agents and wires one into the session.\n" +
				"Switch with Ctrl-a A (or cycle with Ctrl-a N); your choice is remembered\n" +
				"per project. Use `karya dev -a none` for a plain shell.",
			Expect: "karya agent status",
			Hint:   "type it exactly: karya agent status",
			Run:    verifyAgents,
		},
		{
			Title: "Human-in-the-loop tasks",
			Body: "karya hands work to an agent as a *task*: a spec contract in your repo\n" +
				"(.karya/tasks/<id>/SPEC.md) that runs in its own isolated git worktree\n" +
				"(branch task/<id>), so nothing touches your real branch until you review\n" +
				"it. `karya task new <slug>` scaffolds a spec, `karya task start <id>`\n" +
				"creates the worktree, and human gates (plan/diff/verify) guard every\n" +
				"step forward; `karya task list` (Ctrl-a T) is the task board. karya will\n" +
				"create and tear down a real isolated worktree now to prove the containment.",
			Expect: "karya task list",
			Hint:   "type it exactly: karya task list",
			Run:    verifyTasks,
		},
		{
			Title: "Where to go next — try the IDE tutorial",
			Body: "That's the core loop, driven by you. Next, learn the editor the same way:\n" +
				"`karya tutorial ide` runs a keystroke-by-keystroke walkthrough inside the\n" +
				"real IDE (nvim, tmux, lazygit, the agent bridge) for a language you pick.\n" +
				"Also handy: `karya docs tutorial`, `karya docs keymaps`, `karya lang`.\n" +
				"When you're ready: `karya install`, then `karya`.",
		},
	}
	for i := range ls {
		ls[i].Num = i + 1
	}
	return ls
}

// Render writes one lesson to w and, when the lesson is self-working, executes
// its check and reports the outcome. It returns false only when a verification
// actually failed (Outcome Fail), so callers can surface a real problem;
// informational notes (a missing optional tool) do not count as failures.
func Render(w io.Writer, sb *Sandbox, l Lesson) bool {
	total := len(Lessons())
	header := fmt.Sprintf("[%d/%d] %s", l.Num, total, l.Title)
	fmt.Fprintf(w, "%s\n%s\n", header, strings.Repeat("─", len(header)))
	fmt.Fprintf(w, "%s\n", l.Body)
	if l.Run == nil {
		fmt.Fprintln(w)
		return true
	}
	outcome, detail := l.Run(sb)
	marker := map[Outcome]string{Pass: "✓", Note: "•", Fail: "✗"}[outcome]
	fmt.Fprintf(w, "\n  %s %s\n\n", marker, detail)
	return outcome != Fail
}

// verifyIsolation checks that every karya path is namespaced under the karya
// prefix and separate from the user's own config, demonstrating the isolation
// guarantee without touching anything.
func verifyIsolation(*Sandbox) (Outcome, string) {
	p := config.Resolve()
	roots := map[string]string{
		"Config": p.Config, "Data": p.Data, "State": p.State, "Cache": p.Cache,
	}
	for name, dir := range roots {
		if filepath.Base(dir) != config.AppName {
			return Fail, fmt.Sprintf("%s dir %q is not namespaced under %q", name, dir, config.AppName)
		}
	}
	// The editor config nests under karya, never the user's ~/.config/nvim.
	if want := filepath.Join(p.Config, "nvim"); p.NvimConfig() != want {
		return Fail, fmt.Sprintf("NvimConfig %q is not inside the karya prefix", p.NvimConfig())
	}
	return Pass, fmt.Sprintf("all state under a %q prefix (e.g. %s)", config.AppName, p.Config)
}

// verifyScaffold generates a real project in the sandbox and confirms its files
// exist, exercising the same code path as `karya new`.
func verifyScaffold(sb *Sandbox) (Outcome, string) {
	spec, err := project.NewSpec("go", "example.com/hello")
	if err != nil {
		return Fail, err.Error()
	}
	dir, err := project.Scaffold(filepath.Join(sb.Dir, "scaffold"), spec)
	if err != nil {
		return Fail, err.Error()
	}
	want := []string{"go.mod", filepath.Join("cmd", spec.Basename, "main.go"), ".gitignore"}
	for _, f := range want {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			return Fail, fmt.Sprintf("expected %s in scaffolded project: %v", f, err)
		}
	}
	return Pass, fmt.Sprintf("created a go project (go.mod, cmd/%s/main.go, .gitignore) at %s", spec.Basename, dir)
}

// verifyGitInit scaffolds a project and initializes it as a git repo, verifying
// the .git directory appears. It degrades to a Note when git is not installed.
func verifyGitInit(sb *Sandbox) (Outcome, string) {
	if _, err := exec.LookPath("git"); err != nil {
		return Note, "git is not installed yet — install it to version your projects"
	}
	spec, err := project.NewSpec("python", "gitdemo")
	if err != nil {
		return Fail, err.Error()
	}
	dir, err := project.Scaffold(filepath.Join(sb.Dir, "git"), spec)
	if err != nil {
		return Fail, err.Error()
	}
	if err := project.GitInit(dir); err != nil {
		return Fail, err.Error()
	}
	if st, err := os.Stat(filepath.Join(dir, ".git")); err != nil || !st.IsDir() {
		return Fail, fmt.Sprintf("expected a .git directory in %s", dir)
	}
	return Pass, "initialized a git repository (.git created)"
}

// verifyTasks proves task-level isolation for real: it creates a git repo in the
// sandbox, adds a task worktree on a namespaced task/<id> branch under a
// sandbox-local root, records the task, and tears it all down — the same
// worktree.Manager the `karya task` commands use. It degrades to a Note when git
// is not installed rather than failing on a machine still being set up.
func verifyTasks(sb *Sandbox) (Outcome, string) {
	if _, err := exec.LookPath("git"); err != nil {
		return Note, "git is not installed yet — install it to use task worktrees"
	}
	r := ship.ExecRunner{}
	repo := filepath.Join(sb.Dir, "taskrepo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		return Fail, err.Error()
	}
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "tutorial@karya.local"},
		{"config", "user.name", "karya tutorial"},
	} {
		if err := r.Run(repo, "git", args...); err != nil {
			return Fail, err.Error()
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("hello\n"), 0o644); err != nil {
		return Fail, err.Error()
	}
	if err := r.Run(repo, "git", "add", "-A"); err != nil {
		return Fail, err.Error()
	}
	if err := r.Run(repo, "git", "commit", "-m", "init"); err != nil {
		return Fail, err.Error()
	}

	mgr := worktree.Manager{Runner: r, Root: filepath.Join(sb.Dir, "worktrees")}
	const id = "demo"
	path, err := mgr.Add(repo, id)
	if err != nil {
		return Fail, err.Error()
	}
	// Isolation: the checkout lives under the sandbox root, not inside the repo.
	if !strings.HasPrefix(path, sb.Dir) || strings.HasPrefix(path, repo) {
		return Fail, fmt.Sprintf("task worktree %q is not isolated from the repo", path)
	}
	store := task.NewStore(repo)
	tk, err := store.Create(id, spec.Template(id), "")
	if err != nil {
		return Fail, err.Error()
	}
	tk.Branch, tk.Worktree = worktree.Branch(id), path
	if err := store.Save(tk); err != nil {
		return Fail, err.Error()
	}
	// The task record lives inside the repo's .karya directory.
	if _, err := os.Stat(store.SpecPath(id)); err != nil {
		return Fail, fmt.Sprintf("task spec not recorded in the repo: %v", err)
	}
	if err := mgr.Remove(repo, id); err != nil {
		return Fail, err.Error()
	}
	if err := store.Delete(id); err != nil {
		return Fail, err.Error()
	}
	return Pass, fmt.Sprintf("created and cleaned up an isolated worktree on branch %s (contained, reviewable)", worktree.Branch(id))
}

// verifyEmbeddedDocs confirms the offline documentation is present in the binary.
func verifyEmbeddedDocs(*Sandbox) (Outcome, string) {
	topics := assets.DocTopics()
	if len(topics) == 0 {
		return Fail, "no documentation is embedded in this build"
	}
	if _, ok := assets.Doc("tutorial"); !ok {
		return Fail, "the tutorial doc is not embedded"
	}
	return Pass, "embedded topics available offline: " + strings.Join(topics, ", ")
}

// verifyTmux reports whether tmux — required for the IDE session — is available.
// A missing tmux is a Note, not a failure: the tutorial guides you to install it
// rather than failing on a machine that is still being set up.
func verifyTmux(*Sandbox) (Outcome, string) {
	if !tmuxx.Available() {
		return Note, "tmux is not installed yet — install it to launch the IDE session"
	}
	return Pass, "tmux is installed — `karya` can launch the IDE session"
}

// verifyAgents reports which coding agents are detected on this machine. Having
// none is a Note (install one when ready), not a failure.
func verifyAgents(*Sandbox) (Outcome, string) {
	detected := agent.Detect()
	if len(detected) == 0 {
		return Note, "no agents detected yet — install any of: " + strings.Join(agent.Known, ", ")
	}
	return Pass, "detected agents: " + strings.Join(detected, ", ")
}
