package spec

import (
	"strings"
	"testing"
)

const validDoc = `---
id: 2026-08-05-add-retry-to-downloader
status: approved
agent: claude
plan: codex
tdd: true
---

## Objective
Retries make the downloader resilient to flaky networks.

## Acceptance criteria
- [ ] ` + "`download`" + ` retries transient 5xx up to 3 times with backoff
- [x] permanent failures never retry
- [ ] ` + "`go test ./internal/tools/ -run Retry`" + ` passes

## Context
internal/tools/download.go; do not touch the checksum order.

## Constraints
No new dependencies.

## Verification
- cmd: go test -race ./internal/tools/...
- cmd: make lint
`

func TestParseValidDoc(t *testing.T) {
	s, err := Parse([]byte(validDoc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if s.ID != "2026-08-05-add-retry-to-downloader" {
		t.Errorf("ID = %q", s.ID)
	}
	if s.Status != "approved" {
		t.Errorf("Status = %q", s.Status)
	}
	if s.Agent != "claude" {
		t.Errorf("Agent = %q", s.Agent)
	}
	if s.Agents["plan"] != "codex" {
		t.Errorf("Agents[plan] = %q", s.Agents["plan"])
	}
	if !s.TDD {
		t.Error("TDD should be true")
	}
	if !strings.Contains(s.Objective, "resilient") {
		t.Errorf("Objective = %q", s.Objective)
	}
	if len(s.Criteria) != 3 {
		t.Fatalf("Criteria = %d, want 3", len(s.Criteria))
	}
	if s.Criteria[0].Checked || !s.Criteria[1].Checked {
		t.Errorf("checked flags = %v/%v", s.Criteria[0].Checked, s.Criteria[1].Checked)
	}
	if !strings.Contains(s.Context, "download.go") {
		t.Errorf("Context = %q", s.Context)
	}
	if !strings.Contains(s.Constraints, "dependencies") {
		t.Errorf("Constraints = %q", s.Constraints)
	}
	if len(s.Verification) != 2 || s.Verification[0] != "go test -race ./internal/tools/..." {
		t.Errorf("Verification = %v", s.Verification)
	}
	if err := s.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		"no front-matter":        "## Objective\nhi\n",
		"unclosed front-matter":  "---\nid: x\n",
		"bad front-matter line":  "---\nno colon here\n---\n",
		"unknown front-matter":   "---\nid: x\nbogus: y\n---\n",
		"unknown section":        "---\nid: x\n---\n\n## Nonsense\nhi\n",
		"non-checkbox criterion": "---\nid: x\n---\n\n## Acceptance criteria\n- not a checkbox\n",
		"body before section":    "---\nid: x\n---\nstray text\n\n## Objective\nhi\n",
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(doc)); err == nil {
				t.Fatalf("Parse(%q) succeeded, want error", name)
			}
		})
	}
}

func TestValidateErrors(t *testing.T) {
	base := func() *Spec {
		s, err := Parse([]byte(validDoc))
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		return s
	}
	t.Run("missing id", func(t *testing.T) {
		s := base()
		s.ID = ""
		if err := s.Validate(); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("bad id", func(t *testing.T) {
		s := base()
		s.ID = "Bad ID!"
		if err := s.Validate(); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("missing objective", func(t *testing.T) {
		s := base()
		s.Objective = ""
		if err := s.Validate(); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("no criteria", func(t *testing.T) {
		s := base()
		s.Criteria = nil
		if err := s.Validate(); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("no verification", func(t *testing.T) {
		s := base()
		s.Verification = nil
		if err := s.Validate(); err == nil {
			t.Fatal("want error")
		}
	})
}

func TestRenderRoundTrip(t *testing.T) {
	s, err := Parse([]byte(validDoc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	again, err := Parse([]byte(s.Render()))
	if err != nil {
		t.Fatalf("Parse(Render): %v", err)
	}
	if err := again.Validate(); err != nil {
		t.Fatalf("Validate(Render): %v", err)
	}
	if again.ID != s.ID || again.Agent != s.Agent || again.TDD != s.TDD ||
		again.Objective != s.Objective || again.Context != s.Context ||
		again.Constraints != s.Constraints ||
		len(again.Criteria) != len(s.Criteria) || len(again.Verification) != len(s.Verification) {
		t.Errorf("round trip mismatch:\n%s", s.Render())
	}
	// A second render must be byte-identical: Render is canonical.
	if s.Render() != again.Render() {
		t.Errorf("Render not stable:\n%s\n---\n%s", s.Render(), again.Render())
	}
}

func TestVerificationBareListItems(t *testing.T) {
	doc := "---\nid: x\n---\n\n## Verification\n- go test ./...\n- cmd: make lint\n"
	s, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(s.Verification) != 2 || s.Verification[0] != "go test ./..." || s.Verification[1] != "make lint" {
		t.Errorf("Verification = %v", s.Verification)
	}
}

func TestFrontMatterCommentStripping(t *testing.T) {
	doc := "---\nid: x # the task id\nagent: \"claude\" # preferred\n---\n"
	s, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if s.ID != "x" {
		t.Errorf("ID = %q", s.ID)
	}
	if s.Agent != "claude" {
		t.Errorf("Agent = %q", s.Agent)
	}
}

func TestTemplateParses(t *testing.T) {
	doc := Template("2026-08-05-demo")
	if !strings.Contains(doc, "id: 2026-08-05-demo") {
		t.Errorf("template missing id:\n%s", doc)
	}
	s, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Template must parse: %v", err)
	}
	if s.Status != "draft" {
		t.Errorf("Status = %q, want draft", s.Status)
	}
	if len(s.Verification) != 1 {
		t.Errorf("Verification = %v", s.Verification)
	}
}

func TestValidID(t *testing.T) {
	valid := []string{"x", "add-retry", "2026-08-05-add-retry-to-downloader", "a1"}
	invalid := []string{"", "A", "-x", "x-", "has space", "has_underscore", "x..y"}
	for _, id := range valid {
		if !ValidID(id) {
			t.Errorf("ValidID(%q) = false, want true", id)
		}
	}
	for _, id := range invalid {
		if ValidID(id) {
			t.Errorf("ValidID(%q) = true, want false", id)
		}
	}
}
