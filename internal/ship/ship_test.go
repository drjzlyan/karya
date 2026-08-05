package ship

import (
	"fmt"
	"strings"
	"testing"
)

// fakeRunner records calls and answers Output from a canned table keyed by the
// joined command, so the git sequence is asserted without touching a real repo.
type fakeRunner struct {
	out  map[string]string
	err  map[string]error
	runs [][]string
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{out: map[string]string{}, err: map[string]error{}}
}

func key(name string, args ...string) string {
	return strings.TrimSpace(name + " " + strings.Join(args, " "))
}

func (f *fakeRunner) Output(dir, name string, args ...string) (string, error) {
	k := key(name, args...)
	return f.out[k], f.err[k]
}

func (f *fakeRunner) Run(dir, name string, args ...string) error {
	f.runs = append(f.runs, append([]string{name}, args...))
	return f.err[key(name, args...)]
}

func (f *fakeRunner) ranPrefix(parts ...string) bool {
	for _, r := range f.runs {
		if len(r) < len(parts) {
			continue
		}
		ok := true
		for i, p := range parts {
			if r[i] != p {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func TestInsideRepo(t *testing.T) {
	fr := newFakeRunner()
	fr.out[key("git", "rev-parse", "--is-inside-work-tree")] = "true\n"
	g := Git{Runner: fr, Dir: "/x"}
	if !g.InsideRepo() {
		t.Error("expected InsideRepo true")
	}

	fr2 := newFakeRunner()
	fr2.err[key("git", "rev-parse", "--is-inside-work-tree")] = fmt.Errorf("not a repo")
	if (Git{Runner: fr2, Dir: "/x"}).InsideRepo() {
		t.Error("expected InsideRepo false outside a repo")
	}
}

func TestHasStaged(t *testing.T) {
	fr := newFakeRunner()
	fr.out[key("git", "diff", "--cached", "--name-only")] = "a.go\nb.go\n"
	g := Git{Runner: fr, Dir: "/x"}
	if ok, err := g.HasStaged(); err != nil || !ok {
		t.Errorf("HasStaged = %v,%v; want true,nil", ok, err)
	}

	fr.out[key("git", "diff", "--cached", "--name-only")] = "\n"
	if ok, _ := g.HasStaged(); ok {
		t.Error("HasStaged should be false with no staged files")
	}
}

func TestCommitUsesMessageFileAndNoVerify(t *testing.T) {
	fr := newFakeRunner()
	g := Git{Runner: fr, Dir: "/x"}
	if err := g.Commit("feat: thing\n\nbody", true); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	// Expect: git commit -F <tmp> --no-verify
	var commit []string
	for _, r := range fr.runs {
		if len(r) >= 2 && r[0] == "git" && r[1] == "commit" {
			commit = r
		}
	}
	if commit == nil {
		t.Fatal("expected a git commit call")
	}
	if commit[2] != "-F" || commit[3] == "" {
		t.Errorf("commit args = %v, want -F <file>", commit)
	}
	if commit[len(commit)-1] != "--no-verify" {
		t.Errorf("expected --no-verify, got %v", commit)
	}
}

func TestPushSetsUpstream(t *testing.T) {
	fr := newFakeRunner()
	fr.out[key("git", "rev-parse", "--abbrev-ref", "HEAD")] = "feature/x\n"
	g := Git{Runner: fr, Dir: "/x"}
	if err := g.Push(); err != nil {
		t.Fatalf("Push: %v", err)
	}
	if !fr.ranPrefix("git", "push", "--set-upstream", "origin", "feature/x") {
		t.Errorf("expected upstream push, got %v", fr.runs)
	}
}

func TestRevParse(t *testing.T) {
	fr := newFakeRunner()
	fr.out[key("git", "rev-parse", "HEAD")] = "abc123\n"
	g := Git{Runner: fr, Dir: "/x"}
	if sha, err := g.RevParse("HEAD"); err != nil || sha != "abc123" {
		t.Errorf("RevParse = %q,%v; want abc123,nil", sha, err)
	}
}

func TestCommitAllSkipsWhenNothingStaged(t *testing.T) {
	fr := newFakeRunner()
	fr.out[key("git", "diff", "--cached", "--name-only")] = "\n" // nothing staged
	g := Git{Runner: fr, Dir: "/x"}
	committed, err := g.CommitAll("checkpoint", false)
	if err != nil {
		t.Fatalf("CommitAll: %v", err)
	}
	if committed {
		t.Error("CommitAll reported a commit with nothing staged")
	}
	if fr.ranPrefix("git", "commit") {
		t.Error("CommitAll must not commit when nothing is staged")
	}
}

func TestCommitAllCommitsWhenStaged(t *testing.T) {
	fr := newFakeRunner()
	fr.out[key("git", "diff", "--cached", "--name-only")] = "a.go\n"
	g := Git{Runner: fr, Dir: "/x"}
	committed, err := g.CommitAll("checkpoint", false)
	if err != nil || !committed {
		t.Fatalf("CommitAll = %v,%v; want true,nil", committed, err)
	}
	if !fr.ranPrefix("git", "add", "-A") || !fr.ranPrefix("git", "commit") {
		t.Errorf("CommitAll should stage and commit; got %v", fr.runs)
	}
}

func TestMergeNoFF(t *testing.T) {
	fr := newFakeRunner()
	g := Git{Runner: fr, Dir: "/x"}
	if err := g.Merge("karya/t1", true); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if !fr.ranPrefix("git", "merge", "--no-ff", "karya/t1") {
		t.Errorf("expected no-ff merge, got %v", fr.runs)
	}
}

func TestResetHard(t *testing.T) {
	fr := newFakeRunner()
	g := Git{Runner: fr, Dir: "/x"}
	if err := g.ResetHard("deadbeef"); err != nil {
		t.Fatalf("ResetHard: %v", err)
	}
	if !fr.ranPrefix("git", "reset", "--hard", "deadbeef") {
		t.Errorf("expected reset --hard, got %v", fr.runs)
	}
}

func TestSanitizeMessage(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"plain", "feat: x\n\nbody", "feat: x\n\nbody"},
		{"fenced", "```\nfeat: x\n\nbody\n```", "feat: x\n\nbody"},
		{"fenced with lang", "```text\nfix: y\n```\n", "fix: y"},
		{"leading blanks", "\n\nfeat: z", "feat: z"},
		{"only fence", "```\n```", ""},
		{"empty", "   ", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeMessage(tt.in); got != tt.want {
				t.Errorf("SanitizeMessage(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSubjectAndPrompt(t *testing.T) {
	if got := Subject("feat: x\n\nbody"); got != "feat: x" {
		t.Errorf("Subject = %q", got)
	}
	if got := Subject("only line"); got != "only line" {
		t.Errorf("Subject single = %q", got)
	}
	if p := BuildPrompt("DIFFDATA"); !strings.Contains(p, "DIFFDATA") || !strings.Contains(p, "Conventional Commits") {
		t.Errorf("BuildPrompt missing diff or instruction: %q", p)
	}
}
