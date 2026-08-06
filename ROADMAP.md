# karya — Roadmap to v1.0

Phased build order for the human-in-the-loop agent IDE. karya is a **single
self-contained process** that owns the terminal (its own window/pane manager,
git UI, and review surfaces) and embeds Neovim as the text-editing engine over
msgpack-RPC — everything under one unified keymap. Each phase is shippable and
leaves the binary working. Full design: [DESIGN.md](DESIGN.md). Founding
decision: [ADR 0001](docs/adr/0001-single-process-tui-embed-neovim.md). Live
status: [PROGRESS.md](PROGRESS.md). The v0 roadmap (Phases 0–8, shipped) is in
[archive/v0/ROADMAP.md](archive/v0/ROADMAP.md).

Legend: ☐ not started · ◐ in progress · ☑ done

> **The pivot.** The task engine (Phase A, shipped) and the agent adapter layer
> (Phase B, headless) are reused unchanged. What changed is the presentation
> layer: instead of orchestrating an external tmux + a standalone Neovim UI +
> lazygit — three tools with three keymaps — karya now draws its own screen.
> Phases 1–3 build that single-process TUI; the workflow phases (verification,
> marketplaces, hardening) follow.

---

## Phase A — Task engine foundation ☑
**Goal:** the task is a real, persistent, isolated unit of work.

- ☑ `internal/task` — lifecycle, `STATE.json`, artifact store (`.karya/tasks/<id>/`)
- ☑ `internal/spec` — spec parse/validate/render (front-matter + Objective /
  Acceptance criteria / Context / Constraints / Verification)
- ☑ `internal/worktree` — git worktree create/lock/teardown per task, branch
  `task/<id>`, base-ref selection, dirty-tree safety
- ☑ `karya task new|list|status|show|start|abandon`; `karya init`
- ☑ Integration tests: real `git worktree` in `t.TempDir()` repos

**Done:** a task is created from a spec, gets an isolated worktree, and its state
survives restarts — with zero effect on the user's working tree.

---

## Phase B — Agent adapter layer ◐
**Goal:** one interface drives every coding-agent CLI, headless-first. Headless
and unaffected by the UI pivot; proceeds in parallel with Phase 1+.

- ◐ `internal/agentrun` — `Agent` interface, `Caps` matrix, transcripts to task dir
  (WIP carried over; complete the `defaultExec` seam + adapters)
- ◐ Adapters: claude, codex, crush, gemini, aider, copilot + generic shell adapter
- ☐ Plan-mode mapping: native where available, prompt-scaffold emulation otherwise
- ◐ `internal/prompts` — step prompt assembly (spec + feedback + repo context)
- ☐ Adapter contract tests via scripted fake-agent binary; opt-in `-tags=e2e`
  real-CLI smoke suite
- ☑ Fold v0 `internal/ship` into `agentrun` (rename done)

**Done when:** `karya plan <id>` and `karya implement <id>` run any detected
agent headlessly in the task worktree, capturing PLAN.md and transcripts.

---

## Phase 0 — Design pivot (docs first) ◐
**Goal:** the design of record describes the single-process TUI IDE before any
runtime code is written.

- ☑ Rewrite DESIGN.md: orchestrator → single process; embedded-Neovim engine;
  unified keymap; new package map; removed packages; TUI testing strategy
- ◐ Rewrite ROADMAP.md + PROGRESS.md for the new phases
- ☐ Update AGENTS.md: single-process architecture; drop tmux socket, keep
  `NVIM_APPNAME`; stdlib-only reaffirmed; where-things-live
- ☐ Rewrite `docs/keymaps.md` (one unified leader table) + `docs/tutorial.md`
- ☐ Add `docs/adr/0001-single-process-tui-embed-neovim.md`
- ☐ `make sync-docs`; docs-drift test green

**Done when:** docs describe the target architecture and the drift test passes.

---

## Phase 1 — TUI walking skeleton ☑
**Goal:** karya renders its own screen and routes input under one leader — no
editor yet, but splittable, focusable, resizable panes with a live shell.

- ☑ `internal/term` — raw mode (build-tagged darwin/linux termios), ANSI output,
  terminfo-lite capability table, size, input `Decoder` (Key/Resize/Mouse/Paste;
  CSI/SS3/UTF-8, lone-Esc timeout)
- ☑ `internal/cellbuf` — styled cell grid, wide-rune width, minimal `Diff` renderer
- ☑ `internal/tui` — Elm-style `Model/Update/View`, `Program` loop, frame
  coalescing, SIGWINCH, panic-safe restore
- ☑ `internal/keymap` — unified engine: data-driven bindings, modal resolution,
  which-key candidates (single leader `Ctrl-Space`, always intercepted)
- ☑ `internal/layout` — tab/pane tree: splits, focus-by-adjacency, resize, rects
- ☑ `internal/pty` (+ `pty/vt`) — PTY host + VT parser; live shell panes
- ☑ `internal/ide` + `karya tui` launches the TUI (bare `karya` flips in Phase 3,
  once the editor is embedded)

- ☑ Tests: decoder tables, cellbuf diff, keymap resolution, layout geometry,
  tui loop over pipes, vt snapshots, ide model tests, and an integration PTY
  smoke test (real binary renders + quits on Ctrl-Space Q)

**Done when:** `karya tui` opens its own multi-pane TUI; `Ctrl-Space |/-` split,
`Ctrl-Space h/j/k/l` focus, `Ctrl-Space H/J/K/L` resize, which-key discovery —
with working shell panes, all under one keymap. **Shipped 2026-08-06.**

---

## Phase 2 — Embed Neovim as the editing engine ☑
**Goal:** real editing inside a karya pane via msgpack-RPC.

- ☑ `internal/nvimrpc/msgpack` — minimal stdlib msgpack codec (Marshal + streaming
  Decoder) for the nvim RPC subset
- ☑ `internal/nvimrpc` — spawn `nvim --embed`, RPC request/notify + reader,
  `nvim_ui_attach`, `redraw` → `Grid` reducer → `cellbuf`, `nvim_input`,
  `nvim_ui_try_resize`, chrome-off options
- ☑ Editor pane wired into `internal/layout`/`internal/ide`; `karya tui <file>`
  opens the file in the embedded editor
- ☑ Tests: msgpack round-trip, `Grid` reducer snapshots from synthetic batches,
  fake-peer client tests (-race); `-tags=integration` real-nvim (typing renders
  to Grid) + PTY smoke (open file, quit)
- ☑ Slim, plugin-free engine config (`internal/assets/nvimengine/init.lua`:
  options + syntax + treesitter + native LSP + completion) loaded under an
  isolated `karya/nvim-engine` app-name instead of `--clean`, so LSP/treesitter
  work in the embed with no plugin/network bootstrap
- ☑ Configurable leader (`KARYA_LEADER`) — Ctrl+Space is unreliable on macOS
- ☑ Flip bare `karya` and `karya edit` to the TUI (`karya dev` stays the explicit
  legacy tmux launcher until the Phase 7 removal)
- ☑ Zero-setup LSP: opening a file auto-installs its server/formatter/linter into
  karya's isolated prefix in the background (reuses the mise + tool catalog),
  attaches on ready; managed PATH threaded into the embed
- ☐ `.karya/project.toml` LSP override (needs a stdlib TOML reader) — deferred to
  the marketplace phases; auto-detect covers the common case now

**Done when:** editing (with LSP/treesitter) works inside the embedded editor
pane, forwarded by karya's keymap, with no Neovim keymap/UI surface of its own.
**Shipped 2026-08-06.**

---

## Phase 3 — Panes, git panel, and task/gate/review views
**Goal:** the full IDE surface, every part under the one keymap.

- ☐ `internal/git` (headless) + `internal/gitui` — status/stage/commit/branch/
  diff/log/push (replaces lazygit)
- ☐ `internal/diffview` — unified/side-by-side diff renderer (shared)
- ☐ `internal/taskview` (task board), `internal/gateview` (gate inbox),
  `internal/reviewview` + `internal/review` (review layout)
- ☐ `internal/gate` — gate model, approve/reject-with-feedback, delegation, audit
- ☐ Agent CLIs as PTY panes bound to task worktrees (replaces tmux pane switching)
- ☐ `karya review <id>`, `karya gate list|approve|reject|delegate`
- ☐ Tests: git service (fake `Runner` + real-git integration), view model tests,
  layout snapshots, e2e (task → board → review → approve via keys)

**Done when:** the full loop spec → plan → gate → implement → gate runs with
human approvals in the TUI, git is a native panel, and every crossing is recorded.

---

## Phase 4 — Verification & merge
**Goal:** karya certifies; agents never self-certify.

- ☐ Executable `Verification` blocks run in the task worktree → `VERIFY-<n>.md`
- ☐ `tdd: true` acceptance-test-first flow with failure-signature check
- ☐ Cross-agent reviewer step (implementer ≠ reviewer) as pre-gate filter
- ☐ Regression net: auto-detected per-language fast suite at the verify gate
- ☐ `karya verify <id>` + `karya merge <id>` (merge or PR mode, post-gate only)
- ☐ Performance benchmarks in CI with checked-in baselines (DESIGN.md §8.4)

**Done when:** a task reaches DONE only through verify-gate evidence, and merge
lands on the user's terms (direct merge or PR).

---

## Phase 5 — Skills marketplace
**Goal:** portable SKILL.md packages, installed once, visible to every agent.

- ☐ `internal/skills` — registry client, hash-verified install into karya prefix
- ☐ Default registry + `karya skills registry add <url>`
- ☐ Per-agent materialization (opt-in) + removal on uninstall
- ☐ Project-local `.karya/skills/` auto-visible to task agents
- ☐ `karya skills search|install|remove|list` + `internal/skillsview` TUI browser
- ☐ `karya doctor` reports installed skills per agent

**Done when:** a skill installs from the registry and is usable by every opted-in
agent in task runs, fully inside karya-owned paths.

---

## Phase 6 — MCP marketplace
**Goal:** one MCP install → every agent's native config.

- ☐ `internal/mcp` — registry client, runtime provisioning via isolated mise
- ☐ `mcp.toml` source of truth (global + per-project `.karya/mcp.toml`)
- ☐ Native config renderers: claude `.mcp.json`, crush `crush.json`, gemini
  `settings.json`, …; regenerate on agent add/remove
- ☐ Permission scoping per project; secrets by env-var reference only
- ☐ `karya mcp search|install|remove|list|sync` + `internal/mcpview` TUI browser

**Done when:** installing an MCP server makes it available to all detected agents
without the user editing any agent config by hand.

---

## Phase 7 — Hardening, sandbox, v1.0
**Goal:** production-trustworthy HITL IDE.

- ☐ `internal/sandbox` — seatbelt (macOS) / bubblewrap (Linux) confinement of
  agent processes to the task worktree
- ☐ Registry signing verification (cosign) for skills + MCP
- ☐ `karya task audit` — full gate/delegation history report
- ☐ Perf benchmarks green against the budgets (DESIGN.md §8.4)
- ☐ Remove every remaining tmux/lazygit reference; `doctor` checks nvim-embed + PTY
- ☐ Dogfood: karya developed through karya tasks for one full cycle
- ☐ Docs completion: tutorial + keymaps + ADRs for all phases
- ☐ Tag v1.0

**Done when:** sandboxed task execution, signed marketplace content, audit trail,
and docs that match behavior. Ship v1.0.
