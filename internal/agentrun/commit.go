package agentrun

import "strings"

// promptPreamble is the instruction handed to the agent to author a commit
// message from a staged diff. Kept small and explicit so any agent — headless
// or in-pane — produces a clean Conventional-Commit message and nothing else.
const promptPreamble = `Write a single Conventional Commits message for the staged changes below.
Rules: one short imperative subject line (<=72 chars), a blank line, then a concise body explaining the why.
Respond with ONLY the commit message — no code fences, no preamble, no trailing commentary.

Staged diff:
`

// BuildCommitPrompt renders the message-authoring prompt for a staged diff.
func BuildCommitPrompt(diff string) string {
	return promptPreamble + diff
}

// SanitizeMessage cleans an agent's reply into a usable commit message: it strips
// surrounding Markdown code fences, drops a leading conversational line when the
// agent prefixed one, and trims blank padding. It never invents content — an
// empty or fence-only reply yields "".
func SanitizeMessage(raw string) string {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")

	// Drop leading blank lines and a leading ``` fence.
	for len(lines) > 0 && (strings.TrimSpace(lines[0]) == "" || strings.HasPrefix(strings.TrimSpace(lines[0]), "```")) {
		lines = lines[1:]
	}
	// Drop a trailing fence and trailing blank lines.
	for len(lines) > 0 {
		last := strings.TrimSpace(lines[len(lines)-1])
		if last == "" || strings.HasPrefix(last, "```") {
			lines = lines[:len(lines)-1]
			continue
		}
		break
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// Subject returns the first line of a commit message, for use as a PR title.
func Subject(message string) string {
	first, _, _ := strings.Cut(message, "\n")
	return strings.TrimSpace(first)
}
