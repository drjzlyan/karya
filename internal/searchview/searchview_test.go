package searchview

import (
	"strings"
	"testing"

	"github.com/drjzlyan/karya/internal/cellbuf"
	"github.com/drjzlyan/karya/internal/term"
)

func TestParse(t *testing.T) {
	out := "internal/x.go:12:  foo := bar\n" +
		"cmd/main.go:3:package main\n" +
		"bad-line-no-colons\n"
	m := parse(out)
	if len(m) != 2 {
		t.Fatalf("want 2 matches, got %d: %+v", len(m), m)
	}
	if m[0].File != "internal/x.go" || m[0].Line != 12 || !strings.Contains(m[0].Text, "foo := bar") {
		t.Fatalf("first match wrong: %+v", m[0])
	}
	if m[1].File != "cmd/main.go" || m[1].Line != 3 {
		t.Fatalf("second match wrong: %+v", m[1])
	}
}

func fakeSearcher(_, query string) []Match {
	if query != "foo" {
		return nil
	}
	return []Match{
		{File: "a.go", Line: 1, Text: "foo one"},
		{File: "b.go", Line: 9, Text: "foo two"},
	}
}

func TestSearchFlow(t *testing.T) {
	s := New(".", fakeSearcher)
	// Type the query.
	for _, r := range "foo" {
		s.HandleKey(term.RuneKey(r))
	}
	if s.mode != modeInput {
		t.Fatal("typing should stay in input mode")
	}
	// Enter runs the search and switches to results.
	s.HandleKey(term.Named(term.SymEnter))
	if s.mode != modeResults || len(s.results) != 2 {
		t.Fatalf("expected 2 results in results mode, got mode=%d n=%d", s.mode, len(s.results))
	}
	// Navigate + open.
	s.HandleKey(term.RuneKey('j'))
	s.HandleKey(term.Named(term.SymEnter))
	m := s.OpenRequest()
	if m == nil || m.File != "b.go" || m.Line != 9 {
		t.Fatalf("open request = %+v", m)
	}
	if s.OpenRequest() != nil {
		t.Fatal("open request should be consumed")
	}
}

func TestSearchNoResultsStaysInput(t *testing.T) {
	s := New(".", fakeSearcher)
	for _, r := range "zzz" {
		s.HandleKey(term.RuneKey(r))
	}
	s.HandleKey(term.Named(term.SymEnter))
	if s.mode != modeInput {
		t.Fatal("no results should stay in input mode")
	}
}

func TestSearchEscCloses(t *testing.T) {
	s := New(".", fakeSearcher)
	s.HandleKey(term.Named(term.SymEsc))
	if !s.Done() {
		t.Fatal("esc in input mode should close")
	}
}

func TestSearchResultsEscBackToInput(t *testing.T) {
	s := New(".", fakeSearcher)
	for _, r := range "foo" {
		s.HandleKey(term.RuneKey(r))
	}
	s.HandleKey(term.Named(term.SymEnter)) // -> results
	s.HandleKey(term.Named(term.SymEsc))   // back to input
	if s.mode != modeInput || s.Done() {
		t.Fatal("esc in results should return to input, not close")
	}
}

func TestSearchViewRenders(t *testing.T) {
	s := New(".", fakeSearcher)
	for _, r := range "foo" {
		s.HandleKey(term.RuneKey(r))
	}
	s.HandleKey(term.Named(term.SymEnter))
	buf := cellbuf.New(50, 8)
	s.View(buf, cellbuf.Rect{X: 0, Y: 0, W: 50, H: 8}, true)
	out := buf.String()
	if !strings.Contains(out, "search: foo") || !strings.Contains(out, "a.go:1:") {
		t.Fatalf("search render wrong:\n%s", out)
	}
}
