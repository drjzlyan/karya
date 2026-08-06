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

**Active phase:** Phase 0 — Design pivot (docs), then Phase 1 — TUI walking
skeleton. Phase B (agent adapters) proceeds in parallel (headless, unaffected).
**Architecture pivot (2026-08-06):** karya moves from an *orchestrator* (external
tmux + standalone Neovim UI + lazygit, three keymaps) to a **single-process TUI**
that owns the terminal and embeds Neovim as the editing engine over msgpack-RPC,
under one unified keymap. Locked with the user: embed Neovim via RPC; stdlib-only.
See [ADR 0001](docs/adr/0001-single-process-tui-embed-neovim.md).
**Phase A (task engine foundation) shipped** 2026-08-06 (on `main` via #58): spec
contracts, the gate state machine with `STATE.json` audit trail, `task/<id>`
worktrees, the `karya task` CLI, and `karya init`. Reused unchanged by the pivot.
**v0 (Phases 0–8) shipped** through v0.6.0: isolated tmux IDE session, embedded
Neovim config, agent pane management, six-language scaffolding + toolchains,
self-update, install/uninstall. The isolation model + task engine are the
foundation the pivot builds on; the tmux/nvim-UI/lazygit presentation layer is
being replaced. v0 planning docs are archived in `archive/v0/`.

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

### 2026-08-06 — Architecture pivot: single-process TUI IDE

- **Direction change (user-approved):** karya stops orchestrating tmux + a
  standalone Neovim UI + lazygit (each with its own keymap) and becomes a
  **single process** that draws its own screen: its own window/pane/tab manager,
  git panel, task/gate/review views, and PTY-hosted shells/agents — with Neovim
  **embedded as the editing engine** over msgpack-RPC (karya renders Neovim's
  grid into its own cell buffer and owns all input). One leader (`Ctrl-Space`),
  one keymap grammar for pane nav/resize/splits/tabs/git/tasks/gates.
- **Two locked decisions:** (1) embed Neovim via `nvim --embed` RPC rather than
  building an editor from scratch; (2) stdlib-only — build the cell buffer +
  diff renderer, ANSI/terminfo terminal I/O, PTY host, and msgpack-RPC client
  ourselves. No bubbletea, no pty/msgpack libraries.
- **Docs (Phase 0) rewritten:** DESIGN.md (single-process runtime, unified keymap,
  embedded-Neovim engine, new package map, removed packages, four-level TUI
  testing), ROADMAP.md (new Phases 0–7; Phase B reframed as parallel/headless),
  this PROGRESS.md, and (in progress) AGENTS.md + docs/keymaps.md + docs/tutorial.md
  + ADR 0001.
- **Branch:** work continues on `feat/single-process-tui-ide` (from `main`); a
  single PR is raised when the phases complete. In-progress Phase B `agentrun`
  work was carried over and preserved in a `wip` commit (does not yet build —
  `defaultExec` seam pending).

### Resume point (do this next)

1. Finish Phase 0 docs: AGENTS.md (single-process rules, drop tmux socket, keep
   `NVIM_APPNAME`, stdlib-only), `docs/keymaps.md` (one unified `Ctrl-Space`
   table), `docs/tutorial.md` (new UX), ADR 0001; `make sync-docs`; drift test.
2. Phase 1 — TUI walking skeleton: `internal/term`, `internal/cellbuf`,
   `internal/tui`, `internal/keymap`, `internal/layout`, `internal/pty` (+`vt`).
   Bare `karya` launches the TUI with splits/focus/resize + a shell pane, all
   under `Ctrl-Space`. TDD: decoder/diff/keymap/geometry tests + one PTY smoke.
3. In parallel (Phase B, headless): complete `internal/agentrun` — the
   `defaultExec` seam, adapters, `Caps` matrix, prompt assembly; `karya plan
   <id>` / `karya implement <id>`; adapter contract tests via a scripted
   fake-agent binary.
