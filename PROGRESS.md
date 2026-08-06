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

**Active phase:** Phase A — Task engine foundation (in progress).
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

### Resume point (do this next)

1. Phase A: `internal/spec` parse/validate (pure, hermetic) first; then rework
   `internal/task` onto `STATE.json` + the gate state machine (DESIGN.md §2) and
   `internal/worktree` onto in-repo `.karya/` tasks with `task/<id>` branches;
   worktree integration tests in `t.TempDir()` repos.
2. `karya task new|list|status|show|start|abandon` CLI per DESIGN.md §12 (TUI
   arrives in Phase C); the 9–14 `karya task` surface is reshaped to match.
3. `karya init` scaffolding of `.karya/` + repo `AGENTS.md`.
