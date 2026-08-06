package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/drjzlyan/karya/internal/git"
	"github.com/drjzlyan/karya/internal/task"
	"github.com/drjzlyan/karya/internal/verify"
	"github.com/drjzlyan/karya/internal/worktree"
)

// cmdVerify implements `karya verify <id>`: it runs the spec's Verification
// commands in the task worktree and records the result as VERIFY-<n>.md
// evidence. It does not cross the verify gate — the human does that with
// `karya gate approve` after reading the evidence (DESIGN.md §7).
func cmdVerify(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: karya verify <id>")
		return 2
	}
	a, err := newApp()
	if err != nil {
		return fail(err)
	}
	_, store, _, err := taskContext(a)
	if err != nil {
		return fail(err)
	}
	id := args[0]
	t, err := store.Get(id)
	if err != nil {
		return fail(err)
	}
	if t.Worktree == "" {
		fmt.Fprintf(os.Stderr, "task %s has no worktree — run `karya task start %s` first\n", id, id)
		return 2
	}
	sp, err := store.Spec(id)
	if err != nil {
		return fail(err)
	}
	if len(sp.Verification) == 0 {
		fmt.Fprintln(os.Stderr, "spec has no Verification commands")
		return 2
	}

	fmt.Printf("Verifying %s in %s …\n", id, t.Worktree)
	res := verify.Run(t.Worktree, sp.Verification, verify.ShellRunner{})
	path, werr := writeEvidence(store.Dir(id), res.Markdown())
	if werr != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write evidence: %v\n", werr)
	}
	fmt.Println(res.Summary())
	if path != "" {
		fmt.Println("evidence:", path)
	}
	if res.Passed() {
		fmt.Printf("approve the verify gate: karya gate approve %s\n", id)
		return 0
	}
	return 1
}

// writeEvidence writes report to the next VERIFY-<n>.md in dir and returns its
// path.
func writeEvidence(dir, report string) (string, error) {
	n := 1
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "VERIFY-") && strings.HasSuffix(e.Name(), ".md") {
				n++
			}
		}
	}
	path := filepath.Join(dir, fmt.Sprintf("VERIFY-%d.md", n))
	return path, os.WriteFile(path, []byte(report), 0o644)
}

// cmdMerge implements `karya merge <id>`: after the verify gate (state
// merging), it merges the task branch into the current branch (or opens a PR
// with --pr), transitions the task to done, and tears down the worktree. Agents
// never merge; only this post-gate crossing does (DESIGN.md §4).
func cmdMerge(args []string) int {
	fs := flag.NewFlagSet("merge", flag.ContinueOnError)
	pr := fs.Bool("pr", false, "open a pull request instead of merging locally")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: karya merge <id> [--pr]")
		return 2
	}
	a, err := newApp()
	if err != nil {
		return fail(err)
	}
	mgr, store, top, err := taskContext(a)
	if err != nil {
		return fail(err)
	}
	id := fs.Arg(0)
	t, err := store.Get(id)
	if err != nil {
		return fail(err)
	}
	if t.State != task.StateMerging {
		fmt.Fprintf(os.Stderr, "task %s must pass the verify gate first (state: %s)\n", id, t.State)
		return 2
	}
	branch := t.Branch
	if branch == "" {
		branch = worktree.Branch(id)
	}

	if *pr {
		if code := openPR(top, branch, id); code != 0 {
			return code
		}
	} else {
		if err := git.New(top, nil).Merge(branch, "karya: merge task "+id); err != nil {
			return fail(err)
		}
		fmt.Printf("merged %s into the current branch\n", branch)
	}

	if err := t.Transition(task.StateDone, "human", ""); err != nil {
		return fail(err)
	}
	if err := store.Save(t); err != nil {
		return fail(err)
	}
	if err := mgr.Remove(top, id); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not remove worktree: %v\n", err)
	}
	fmt.Printf("task %s → done\n", id)
	return 0
}

// openPR pushes the task branch and opens a PR with gh (best-effort).
func openPR(top, branch, id string) int {
	r := git.ExecRunner{}
	if err := r.Run(top, "git", "push", "-u", "origin", branch); err != nil {
		return fail(fmt.Errorf("push %s: %w", branch, err))
	}
	if err := r.Run(top, "gh", "pr", "create", "--fill", "--head", branch); err != nil {
		return fail(fmt.Errorf("gh pr create (is gh installed and authed?): %w", err))
	}
	fmt.Printf("opened a pull request for %s\n", branch)
	return 0
}
