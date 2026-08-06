// Package prompts assembles the step prompts karya hands to coding agents
// (DESIGN.md §5): agents never receive hand-assembled context — every prompt
// is built from the task's spec contract (internal/spec), the repo's
// agent-facing docs (AGENTS.md, .karya/CONTEXT.md), and, on gate rejections,
// the human's feedback. Prompt assembly is pure string building (stdlib only)
// so the exact text every agent sees is exhaustively unit-testable.
package prompts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/drjzlyan/karya/internal/spec"
)

// maxRepoDocBytes caps each repo doc folded into a prompt so a giant AGENTS.md
// cannot swamp the agent's context window; truncation is marked, not silent.
const maxRepoDocBytes = 16 << 10

// repoDocs are the agent-facing docs karya looks for, in priority order.
// AGENTS.md is the repo-wide contract (karya init scaffolds it); CONTEXT.md is
// the per-project karya context (DESIGN.md §10).
var repoDocs = []string{"AGENTS.md", filepath.Join(".karya", "CONTEXT.md")}

// RepoContext reads the agent-facing docs of the repository at root, returning
// them as labeled Markdown sections, or "" when none exist. Best-effort: an
// unreadable doc is skipped, never an error — a missing doc must not block a
// task step.
func RepoContext(root string) string {
	var b strings.Builder
	for _, rel := range repoDocs {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			continue
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}
		if len(content) > maxRepoDocBytes {
			content = content[:maxRepoDocBytes] + "\n\n(truncated)"
		}
		fmt.Fprintf(&b, "### %s\n\n%s\n\n", rel, content)
	}
	return strings.TrimSpace(b.String())
}

// PlanInput carries everything a plan-step prompt needs.
type PlanInput struct {
	Spec        *spec.Spec // the task contract (required)
	RepoContext string     // RepoContext output (may be "")
	Feedback    string     // human feedback from a plan-gate rejection (revision)
}

// Plan renders the plan-step prompt. The plan step is read-only by contract:
// the agent studies the repo and the spec and answers with a Markdown plan —
// the prompt-scaffold that doubles as plan-mode emulation for agents without a
// native read-only mode (DESIGN.md §5).
func Plan(in PlanInput) string {
	var b strings.Builder
	b.WriteString(`You are the planning agent for a karya task. karya is a human-in-the-loop IDE: you plan, a human reviews your plan at a gate, and only then is it implemented.

## Planning rules
- Study the repository before planning; ground every step in the real code.
- Do NOT modify, create, or delete any files. This step is read-only.
- Respond with ONLY the implementation plan as Markdown — no preamble, no commentary.
- The plan must be a numbered list of concrete steps; under each step, name the files/packages it touches and the acceptance criteria it satisfies.
- End with a short "Risks & assumptions" section.

## The task contract (SPEC.md)

`)
	b.WriteString(in.Spec.Render())
	if in.RepoContext != "" {
		b.WriteString("\n## Repository docs\n\n")
		b.WriteString(in.RepoContext)
		b.WriteString("\n")
	}
	writeFeedback(&b, in.Feedback, "Your previous plan for this task was REJECTED at the plan gate.")
	return b.String()
}

// ImplementInput carries everything an implement-step prompt needs.
type ImplementInput struct {
	Spec        *spec.Spec // the task contract (required)
	Plan        string     // the approved PLAN.md content
	RepoContext string     // RepoContext output (may be "")
	Feedback    string     // human feedback from a diff-gate rejection (revision)
}

// Implement renders the implement-step prompt: the agent works in the task's
// isolated git worktree (its cwd), following the human-approved plan. karya —
// never the agent — commits the work, so the prompt forbids git mutations.
func Implement(in ImplementInput) string {
	var b strings.Builder
	b.WriteString(`You are the implementation agent for a karya task. karya is a human-in-the-loop IDE: your plan was approved by a human; now implement it.

## Implementation rules
- Your working directory is an isolated git worktree for this task; all file changes happen here.
- Follow the approved plan below; satisfy EVERY acceptance criterion in the spec.
- Respect the spec's Constraints exactly — they are binding.
- Do NOT run git commit, git push, or any git history-changing command. karya commits your work; agents never merge.
- Do NOT modify .karya/ — the spec, plan, and state are the human's contract, not your workspace.

## The task contract (SPEC.md)

`)
	b.WriteString(in.Spec.Render())
	b.WriteString("\n## The approved plan (PLAN.md)\n\n")
	b.WriteString(strings.TrimSpace(in.Plan))
	b.WriteString("\n")
	if in.RepoContext != "" {
		b.WriteString("\n## Repository docs\n\n")
		b.WriteString(in.RepoContext)
		b.WriteString("\n")
	}
	writeFeedback(&b, in.Feedback, "Your previous implementation for this task was REJECTED at the diff gate.")
	return b.String()
}

// writeFeedback appends the human's gate feedback to the prompt when this run
// is a revision after a rejection (the core HITL loop, DESIGN.md §2).
func writeFeedback(b *strings.Builder, feedback, lead string) {
	if strings.TrimSpace(feedback) == "" {
		return
	}
	fmt.Fprintf(b, "\n## Human feedback — revise accordingly\n\n%s The human wrote:\n\n> %s\n",
		lead, strings.ReplaceAll(strings.TrimSpace(feedback), "\n", "\n> "))
}
