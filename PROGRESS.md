# karya — Progress Log

Living status document. **Read this first when resuming work.** It records what
is done, what is in flight, and the exact next action. Update it at the end of
every working session.

- **Design:** [DESIGN.md](DESIGN.md)
- **Roadmap:** [ROADMAP.md](ROADMAP.md)
- **Agent/dev guide:** [AGENTS.md](AGENTS.md)
- **v0 history:** [archive/v0/](archive/v0/)

---

## Current status

**Active phase:** Phase B — Agent adapter layer (not started).
**Phase A (task engine foundation) shipped** 2026-08-06: spec contracts, the
gate state machine with `STATE.json` audit trail, `task/<id>` worktrees, the
`karya task` CLI, and `karya init`.
**v0 (Phases 0–8) shipped** through v0.6.0: isolated tmux IDE session, embedded
Neovim config, agent pane management, six-language scaffolding + toolchains,
self-update, install/uninstall. That work is the foundation v1.0 builds on; its
planning docs are archived in `archive/v0/`.

**Already on main from the earlier 9–14 arc (superseded direction; code kept
and adapted into the phases below):** pluggable `agent.Runner` (becomes Phase
B's adapter seam), a first `internal/task` + `internal/worktree` (tasks stored
under the karya prefix on `karya/<id>` branches — Phase A moves them to in-repo
`.karya/tasks/<id>/` on `task/<id>` branches with spec contracts and
`STATE.json`), a native Claude-API engine (`internal/native`), and a task fleet
dashboard.

### 2026-08-05 — v1.0 replan: human-in-the-loop agent IDE

- Product direction reset: from "AI-first terminal IDE with an agent pane" to
  **human-in-the-loop, agent-based IDE** — tasks with spec contracts, isolated
  git worktrees, mandatory human gates (plan / diff / verification), blended
  agent CLIs behind one adapter layer, skills + MCP marketplaces, and a
  karya-native modal TUI (Neovim-inspired keymaps, tested like a backend).
- New [DESIGN.md](DESIGN.md) written: task state machine, spec format
  (objective + executable acceptance criteria), two-level isolation, agent
  adapter interface, review UX, TUI testing strategy (model/snapshot/PTY/e2e),
  performance budget, marketplace trust model, migration from v0.
- Old planning docs archived to `archive/v0/` (PLAN, ROADMAP, PROGRESS,
  TOOLING_REFACTOR ×2); ROADMAP.md and PROGRESS.md reset for v1.0.
- AGENTS.md merged (single file; old AGENT.md removed) and repointed at
  DESIGN.md.

### 2026-08-06 — Phase A: task engine foundation

- **`internal/spec`** (new) — hand-parsed, stdlib-only SPEC.md format per
  DESIGN.md §3: front-matter (id/status/agent/per-step pins/tdd) + Objective /
  Acceptance criteria (checkboxes) / Context / Constraints / Verification
  (`cmd:` list). Parse/Validate/Render (canonical, round-trip stable) +
  Template. Hermetic table tests.
- **`internal/task`** (reworked) — tasks moved from the 9–14-era karya-prefix
  JSON store to in-repo `.karya/tasks/<id>/` (SPEC.md + STATE.json). Gate state
  machine per DESIGN.md §2 (draft→planned→approved→implementing→verifying→
  merging→done, rejections loop back with mandatory feedback, every crossing
  records actor/gate/timestamp). `EnsureProjectDir` installs a
  `.karya/.gitignore` that keeps runtime state local but SPEC.md committable
  (verified against real git).
- **`internal/worktree`** (reworked) — branch prefix `karya/` → `task/`;
  `AddFrom` adds base-ref selection (dirty-tree safety: uncommitted changes
  never leak into a task). `Remove` prunes the empty per-project dir. New
  integration test proves base-ref + dirty-tree containment with real git.
- **CLI reshaped** — `karya task new|list|status|show|start|abandon` per
  DESIGN.md §12; `karya init` scaffolds `.karya/` + a toolchain-detected repo
  AGENTS.md. Removed the 9–14 surface (dashboard/switch/plan/approve-plan/
  review/merge/reject/checkpoint/rewind/allow) — agent-driven steps return in
  Phases B–D. `taskContext` resolves the caller's repo first (a pane cd'd
  outside the project no longer scatters tasks into the session's repo).
- **Editor/tmux surface** — karyatasks.lua remapped (`<leader>kn/kl/ks/kt/ka`),
  keymap guard updated, `Ctrl-a T` popup now shows `task list` (real task TUI
  in Phase C). Tutorial lesson + docs (commands/keymaps/tutorial) rewritten and
  re-vendored.
- **Drive-by fix:** `shell_test.go` unset `STARSHIP_CONFIG` so the plain-shell
  assertion is hermetic on machines where karya itself is installed.

### Resume point (do this next)

1. Phase B: `internal/agentrun` — the `Agent` interface + `Caps` matrix
   (DESIGN.md §5), reusing the 9–14 `agent.Runner` seam; transcripts into the
   task dir.
2. Adapters for the detected CLIs + generic shell adapter; plan-mode mapping
   (native where available, prompt-scaffold emulation otherwise).
3. `karya plan <id>` / `karya implement <id>` driving an agent headlessly in
   the task worktree, capturing PLAN.md; adapter contract tests via a scripted
   fake-agent binary.
