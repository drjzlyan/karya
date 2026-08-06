// Package spec parses, validates, and renders karya task specs — the
// human↔agent contract of the human-in-the-loop IDE (DESIGN.md §3).
//
// A spec is a Markdown document with a small YAML-like front-matter block and a
// fixed set of sections: Objective, Acceptance criteria (checkboxes), Context,
// Constraints, and Verification (executable commands karya runs at the verify
// gate). Specs live at .karya/tasks/<id>/SPEC.md inside the user's repo: they
// are the one karya artifact meant to be committed, so both humans and agents
// can diff and re-ingest them. The format is hand-parsed (stdlib only) to keep
// karya's zero-external-dependency core (AGENTS.md).
package spec

import (
	"fmt"
	"regexp"
	"strings"
)

// Step agent-pin keys recognized in the front-matter. A spec may route each
// step to a different agent (DESIGN.md §5: "plan: codex, implement: claude").
var stepKeys = []string{"plan", "implement", "review"}

// section names, in canonical render order.
const (
	secObjective     = "Objective"
	secCriteria      = "Acceptance criteria"
	secContext       = "Context"
	secConstraints   = "Constraints"
	secVerification  = "Verification"
	frontMatterDelim = "---"
)

// Criterion is one machine-checkable acceptance criterion: a Markdown checkbox.
type Criterion struct {
	Text    string
	Checked bool
}

// Spec is the parsed form of a task's SPEC.md.
type Spec struct {
	ID           string            // task id, e.g. 2026-08-05-add-retry
	Status       string            // free-form lifecycle label (draft, approved, …)
	Agent        string            // preferred agent for implementation (optional)
	Agents       map[string]string // per-step agent pins: plan/implement/review
	TDD          bool              // acceptance-test-first flow (DESIGN.md §7.1)
	Objective    string
	Criteria     []Criterion
	Context      string
	Constraints  string
	Verification []string // commands karya executes at the verify gate
}

// idPattern allows date-prefixed slugs (2026-08-05-add-retry) and plain slugs.
var idPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// ValidID reports whether s is a usable task id / spec id: lowercase
// alphanumerics and hyphens, no leading/trailing hyphen.
func ValidID(s string) bool { return idPattern.MatchString(s) }

// Parse reads a SPEC.md document. It returns an error for malformed
// front-matter, unknown sections, or criteria that are not checkboxes; it does
// NOT validate completeness — call Validate for that, so callers can parse
// in-progress drafts (e.g. freshly scaffolded templates).
func Parse(data []byte) (*Spec, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(text, frontMatterDelim+"\n") {
		return nil, fmt.Errorf("spec: missing front-matter opening %q", frontMatterDelim)
	}
	rest := text[len(frontMatterDelim)+1:]
	end := strings.Index(rest, "\n"+frontMatterDelim)
	if end < 0 {
		return nil, fmt.Errorf("spec: missing front-matter closing %q", frontMatterDelim)
	}
	fm, body := rest[:end], rest[end+len(frontMatterDelim)+1:]

	s := &Spec{Agents: map[string]string{}}
	if err := parseFrontMatter(s, fm); err != nil {
		return nil, err
	}
	if err := parseBody(s, body); err != nil {
		return nil, err
	}
	return s, nil
}

// parseFrontMatter fills s from the key: value lines of the front-matter block.
func parseFrontMatter(s *Spec, fm string) error {
	for n, line := range strings.Split(fm, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return fmt.Errorf("spec: front-matter line %d is not key: value: %q", n+1, line)
		}
		key, value = strings.TrimSpace(key), stripComment(strings.TrimSpace(value))
		switch key {
		case "id":
			s.ID = value
		case "status":
			s.Status = value
		case "agent":
			s.Agent = value
		case "tdd":
			s.TDD = value == "true"
		default:
			if isStepKey(key) {
				s.Agents[key] = value
				continue
			}
			return fmt.Errorf("spec: unknown front-matter key %q", key)
		}
	}
	return nil
}

// stripComment drops a trailing " # …" comment from a front-matter value.
func stripComment(v string) string {
	if i := strings.Index(v, " #"); i >= 0 {
		v = v[:i]
	}
	// Trim surrounding quotes so `agent: "claude"` reads as claude.
	return strings.Trim(strings.TrimSpace(v), `"'`)
}

func isStepKey(key string) bool {
	for _, k := range stepKeys {
		if k == key {
			return true
		}
	}
	return false
}

// parseBody fills s from the sectioned Markdown body.
func parseBody(s *Spec, body string) error {
	sections := splitSections(body)
	for _, sec := range sections {
		name, content := sec.name, sec.content
		switch name {
		case secObjective:
			s.Objective = strings.TrimSpace(content)
		case secCriteria:
			criteria, err := parseCriteria(content)
			if err != nil {
				return err
			}
			s.Criteria = criteria
		case secContext:
			s.Context = strings.TrimSpace(content)
		case secConstraints:
			s.Constraints = strings.TrimSpace(content)
		case secVerification:
			s.Verification = parseCommands(content)
		case "":
			// Content before the first section header: tolerate whitespace only.
			if strings.TrimSpace(content) != "" {
				return fmt.Errorf("spec: content before the first ## section: %q", firstLine(content))
			}
		default:
			return fmt.Errorf("spec: unknown section %q (want %s / %s / %s / %s / %s)",
				name, secObjective, secCriteria, secContext, secConstraints, secVerification)
		}
	}
	return nil
}

// splitSections groups a Markdown body by its "## " headers, in order.
func splitSections(body string) []sectionKV {
	var out []sectionKV
	name := ""
	var buf strings.Builder
	flush := func() {
		out = append(out, sectionKV{name, buf.String()})
		buf.Reset()
	}
	for _, line := range strings.Split(body, "\n") {
		if h, ok := strings.CutPrefix(line, "## "); ok {
			flush()
			name = strings.TrimSpace(h)
			continue
		}
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	flush()
	return out
}

// sectionKV is one "## " section: its name and raw content.
type sectionKV struct {
	name    string
	content string
}

// parseCriteria reads the "- [ ] …" / "- [x] …" checkbox list.
func parseCriteria(content string) ([]Criterion, error) {
	var out []Criterion
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var c Criterion
		switch {
		case strings.HasPrefix(line, "- [ ] "):
			c = Criterion{Text: strings.TrimSpace(line[len("- [ ] "):])}
		case strings.HasPrefix(line, "- [x] "), strings.HasPrefix(line, "- [X] "):
			c = Criterion{Text: strings.TrimSpace(line[len("- [x] "):]), Checked: true}
		default:
			return nil, fmt.Errorf("spec: acceptance criteria must be checkboxes (- [ ] …), got %q", line)
		}
		if c.Text == "" {
			return nil, fmt.Errorf("spec: empty acceptance criterion")
		}
		out = append(out, c)
	}
	return out, nil
}

// parseCommands reads the verification command list ("- cmd: …" or "- …").
func parseCommands(content string) []string {
	var out []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if cmd, ok := strings.CutPrefix(line, "- cmd:"); ok {
			line = strings.TrimSpace(cmd)
		} else if c, ok := strings.CutPrefix(line, "-"); ok {
			line = strings.TrimSpace(c)
		} else {
			continue
		}
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// Validate checks the spec is complete enough to drive a task: a usable id, an
// objective, at least one acceptance criterion, and at least one verification
// command (the verify gate needs something executable — DESIGN.md §3).
func (s *Spec) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("spec: front-matter id is required")
	}
	if !ValidID(s.ID) {
		return fmt.Errorf("spec: id %q must be lowercase alphanumerics and hyphens", s.ID)
	}
	if s.Objective == "" {
		return fmt.Errorf("spec: Objective section is required")
	}
	if len(s.Criteria) == 0 {
		return fmt.Errorf("spec: at least one acceptance criterion is required")
	}
	if len(s.Verification) == 0 {
		return fmt.Errorf("spec: at least one Verification command is required")
	}
	return nil
}

// Render produces the canonical SPEC.md form: front-matter (id, status, agent,
// per-step pins, tdd) followed by the five sections in order.
func (s *Spec) Render() string {
	var b strings.Builder
	b.WriteString(frontMatterDelim + "\n")
	fmt.Fprintf(&b, "id: %s\n", s.ID)
	status := s.Status
	if status == "" {
		status = "draft"
	}
	fmt.Fprintf(&b, "status: %s\n", status)
	if s.Agent != "" {
		fmt.Fprintf(&b, "agent: %s\n", s.Agent)
	}
	for _, k := range stepKeys {
		if v := s.Agents[k]; v != "" {
			fmt.Fprintf(&b, "%s: %s\n", k, v)
		}
	}
	if s.TDD {
		b.WriteString("tdd: true\n")
	}
	b.WriteString(frontMatterDelim + "\n\n")

	writeSection(&b, secObjective, s.Objective)

	b.WriteString("## " + secCriteria + "\n")
	for _, c := range s.Criteria {
		box := "[ ]"
		if c.Checked {
			box = "[x]"
		}
		fmt.Fprintf(&b, "- %s %s\n", box, c.Text)
	}
	b.WriteString("\n")

	writeSection(&b, secContext, s.Context)
	writeSection(&b, secConstraints, s.Constraints)

	b.WriteString("## " + secVerification + "\n")
	for _, cmd := range s.Verification {
		fmt.Fprintf(&b, "- cmd: %s\n", cmd)
	}
	return b.String()
}

// writeSection emits "## name" followed by content (or a placeholder line when
// empty) and a trailing blank line.
func writeSection(b *strings.Builder, name, content string) {
	b.WriteString("## " + name + "\n")
	if content == "" {
		b.WriteString("(none)\n")
	} else {
		b.WriteString(content + "\n")
	}
	b.WriteString("\n")
}

// Template returns the scaffold `karya task new` writes: a draft spec the human
// fills in. It deliberately does not parse-validate (the placeholders are not
// real criteria yet); it becomes valid as the human edits it.
func Template(id string) string {
	return frontMatterDelim + "\n" +
		"id: " + id + "\n" +
		"status: draft\n" +
		"# agent: claude            # preferred agent for implementation (optional)\n" +
		"# plan: codex              # optional per-step agent pins\n" +
		"# tdd: true                # acceptance-test-first flow\n" +
		frontMatterDelim + "\n\n" +
		"## Objective\n" +
		"One paragraph. What outcome, why it matters.\n\n" +
		"## Acceptance criteria\n" +
		"- [ ] first machine-checkable result (a reviewer can tick this)\n" +
		"- [ ] `command that proves it` passes\n\n" +
		"## Context\n" +
		"Files/packages the agent must read first; what is out of scope.\n\n" +
		"## Constraints\n" +
		"e.g. no new dependencies; public API unchanged.\n\n" +
		"## Verification\n" +
		"- cmd: go test ./...\n"
}

// firstLine returns the first non-empty line of s, for error messages.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return ""
}
