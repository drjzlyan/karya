package git

import (
	"reflect"
	"testing"
)

// fakeRunner records calls and returns scripted output.
type fakeRunner struct {
	outputs map[string]string // key: strings.Join(args," ")
	calls   [][]string
	runErr  error
}

func (f *fakeRunner) key(args []string) string {
	s := ""
	for i, a := range args {
		if i > 0 {
			s += " "
		}
		s += a
	}
	return s
}

func (f *fakeRunner) Output(dir, name string, args ...string) (string, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	return f.outputs[f.key(args)], nil
}

func (f *fakeRunner) Run(dir, name string, args ...string) error {
	f.calls = append(f.calls, append([]string{name}, args...))
	return f.runErr
}

func TestParseStatus(t *testing.T) {
	out := " M internal/git/git.go\n" +
		"A  added.go\n" +
		"?? new.txt\n" +
		"R  old.go -> renamed.go\n" +
		"MM both.go"
	files := parseStatus(out)
	if len(files) != 5 {
		t.Fatalf("want 5 files, got %d: %+v", len(files), files)
	}
	// " M ..." -> unstaged only
	if files[0].Path != "internal/git/git.go" || files[0].Staged() || !files[0].Unstaged() {
		t.Fatalf("modified-unstaged wrong: %+v", files[0])
	}
	// "A  added.go" -> staged only
	if !files[1].Staged() || files[1].Unstaged() {
		t.Fatalf("added-staged wrong: %+v", files[1])
	}
	// untracked
	if !files[2].Untracked || files[2].Path != "new.txt" || !files[2].Unstaged() {
		t.Fatalf("untracked wrong: %+v", files[2])
	}
	// rename -> new path
	if files[3].Path != "renamed.go" || !files[3].Staged() {
		t.Fatalf("rename wrong: %+v", files[3])
	}
	// "MM" -> both staged and unstaged
	if !files[4].Staged() || !files[4].Unstaged() {
		t.Fatalf("MM wrong: %+v", files[4])
	}
}

func TestStatusArgvAndParse(t *testing.T) {
	fr := &fakeRunner{outputs: map[string]string{"status --porcelain": " M a.go"}}
	r := New("/repo", fr)
	files, err := r.Status()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "a.go" {
		t.Fatalf("status parse: %+v", files)
	}
}

func TestStageUnstageArgv(t *testing.T) {
	fr := &fakeRunner{}
	r := New("/repo", fr)
	_ = r.Stage("a.go")
	_ = r.StageAll()
	if !reflect.DeepEqual(fr.calls[0], []string{"git", "add", "--", "a.go"}) {
		t.Fatalf("stage argv: %v", fr.calls[0])
	}
	if !reflect.DeepEqual(fr.calls[1], []string{"git", "add", "-A"}) {
		t.Fatalf("stageall argv: %v", fr.calls[1])
	}
}

func TestDiffArgv(t *testing.T) {
	fr := &fakeRunner{}
	r := New("/repo", fr)
	_, _ = r.Diff("a.go", false)
	_, _ = r.Diff("a.go", true)
	if !reflect.DeepEqual(fr.calls[0], []string{"git", "diff", "--", "a.go"}) {
		t.Fatalf("worktree diff argv: %v", fr.calls[0])
	}
	if !reflect.DeepEqual(fr.calls[1], []string{"git", "diff", "--cached", "--", "a.go"}) {
		t.Fatalf("staged diff argv: %v", fr.calls[1])
	}
}

func TestLogParse(t *testing.T) {
	fr := &fakeRunner{outputs: map[string]string{
		"log -2 --pretty=%h\x1f%s\x1f%an\x1f%cr": "abc123\x1ffirst\x1fAda\x1f2 hours ago\ndef456\x1fsecond\x1fLin\x1f3 days ago",
	}}
	r := New("/repo", fr)
	commits, err := r.Log(2)
	if err != nil {
		t.Fatal(err)
	}
	want := []Commit{
		{Hash: "abc123", Subject: "first", Author: "Ada", When: "2 hours ago"},
		{Hash: "def456", Subject: "second", Author: "Lin", When: "3 days ago"},
	}
	if !reflect.DeepEqual(commits, want) {
		t.Fatalf("log = %+v want %+v", commits, want)
	}
}

func TestCheckoutAndCreateBranch(t *testing.T) {
	fr := &fakeRunner{}
	r := New("/repo", fr)
	if err := r.Checkout("feature"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fr.calls[0], []string{"git", "checkout", "feature"}) {
		t.Fatalf("checkout argv: %v", fr.calls[0])
	}
	if err := r.CreateBranch("new-thing"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fr.calls[1], []string{"git", "checkout", "-b", "new-thing"}) {
		t.Fatalf("create-branch argv: %v", fr.calls[1])
	}
}

func TestStashListParse(t *testing.T) {
	fr := &fakeRunner{outputs: map[string]string{
		"stash list --format=%gd\x1f%gs": "stash@{0}\x1fWIP on main: abc feature\nstash@{1}\x1fOn dev: fix",
	}}
	r := New("/repo", fr)
	stashes, err := r.StashList()
	if err != nil {
		t.Fatal(err)
	}
	want := []Stash{
		{Ref: "stash@{0}", Desc: "WIP on main: abc feature"},
		{Ref: "stash@{1}", Desc: "On dev: fix"},
	}
	if !reflect.DeepEqual(stashes, want) {
		t.Fatalf("stashes = %+v want %+v", stashes, want)
	}
}

func TestStashPushAndPop(t *testing.T) {
	fr := &fakeRunner{}
	r := New("/repo", fr)
	if err := r.Stash(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fr.calls[0], []string{"git", "stash", "push"}) {
		t.Fatalf("stash argv: %v", fr.calls[0])
	}
	if err := r.StashPop("stash@{0}"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fr.calls[1], []string{"git", "stash", "pop", "stash@{0}"}) {
		t.Fatalf("stash pop argv: %v", fr.calls[1])
	}
}
