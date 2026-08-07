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

## Phase 3 — Panes, git panel, and task/gate/review views ◐
**Goal:** the full IDE surface, every part under the one keymap.

- ☑ `internal/git` (headless) + `internal/gitui` — status/stage/commit/diff/log/
  branch/push (replaces lazygit); `Ctrl+Space g g/c/p`
- ☑ `internal/diffview` — unified diff parser + cellbuf renderer (shared)
- ☑ `internal/taskview` (task board, `Ctrl+Space t t`) + `internal/reviewview`
  (scrollable review, `Ctrl+Space r`) + `internal/review` (artifact assembly)
- ☑ `internal/gate` — pending-gate model over the task state machine; crossings
  record actor + feedback in STATE.json (the audit trail)
- ☑ `karya review <id>`, `karya gate list|approve|reject|delegate`
- ☑ Tests: git service (fake `Runner` + real-git integration), view model +
  snapshot tests, gate crossing logic
- ☐ In-TUI approve/reject keys + gate inbox view (`internal/gateview`) — Phase 4
- ☐ Agent CLIs as PTY panes bound to task worktrees — Phase 4 (with agentrun
  interactive runs)

**Done when:** the full loop spec → plan → gate → implement → gate runs with
human approvals, git is a native panel, and every crossing is recorded.
**Core shipped 2026-08-07; in-TUI approve + agent panes move to Phase 4.**

---

## Phase 4 — Verification & merge ◐
**Goal:** karya certifies; agents never self-certify.

- ☑ `internal/verify`: executable `Verification` blocks run in the task worktree,
  captured to `VERIFY-<n>.md` evidence (karya certifies, complete evidence)
- ☑ `karya verify <id>` (records evidence) + `karya merge <id>` (merge or `--pr`,
  post verify-gate only; transitions to done, tears down the worktree)
- ☑ In-TUI approve/reject keys + gate inbox view (`internal/gateview`,
  `Ctrl+Space a`); review view crosses gates in place
- ☑ Agent CLIs as PTY panes bound to task worktrees (task board `a`)
- ☐ `tdd: true` acceptance-test-first flow with failure-signature check
- ☐ Cross-agent reviewer step (implementer ≠ reviewer) as pre-gate filter
- ☐ Regression net: auto-detected per-language fast suite at the verify gate
- ☐ Performance benchmarks in CI with checked-in baselines (DESIGN.md §8.4)

**Done when:** a task reaches DONE only through verify-gate evidence, and merge
lands on the user's terms (direct merge or PR).
**Core shipped 2026-08-07; tdd/cross-agent/regression-net/perf remain.**

---

## Phase 4.5 — Config & continuity ☑
**Goal:** a ready-to-work default view, layered agent instructions, and
agent-agnostic per-task memory so agents are swappable without losing work.

- ☑ Default 3-pane layout on bare `karya`: editor (left) + agent pane (top-right,
  first detected agent, in the repo) + build/test shell (bottom-right); graceful
  fallbacks (DESIGN.md §6.1); config override deferred
- ☑ Layered instructions global → project → task (enhance, not override; opt-in
  `<!-- karya:override -->`): global `instructions.md` under the karya prefix,
  prepended in `internal/prompts` before repo `AGENTS.md`/`.karya/CONTEXT.md`
- ☑ Per-task `MEMORY.md` (agent-agnostic): read into every prompt, append/read
  API, shown in review — so an agent working a task can be replaced mid-task
- ☑ Agents run on karya's tools: test asserts the child env puts karya's managed
  `PATH` ahead of the user's; agent panes/headless runs use the task worktree cwd
- ☐ `karya config edit` for the global instructions file (deferred)

**Done when:** karya opens into the 3-pane view; agent prompts layer global +
project + task context; and swapping the agent on a task preserves continuity.
**Shipped 2026-08-07.**

---

## Phase 4.6 — Human IDE features ☑
**Goal:** karya is a full editor for humans (triage, close reading, manual
fixes), not only an agent surface (DESIGN.md §6.4).

- ☑ Editor LSP navigation (plugin-free, engine config): declaration/type-def,
  document/workspace symbols, diagnostics float + `[d`/`]d` + loclist, signature
  help — with the existing definition/references/implementation/hover/rename/
  code-action/format
- ☑ Fuzzy file finder view (`internal/finder`, `Ctrl+Space f`): ripgrep file
  list (walk fallback), fuzzy filter, Enter opens in the editor pane
- ☑ Project search / live grep view (`internal/searchview`, `Ctrl+Space /`):
  ripgrep matches (file:line), Enter opens at the location
- ☑ `editorPane.OpenFile(path, line)` + focus-editor action (`Ctrl+Space e`)

**Done when:** a human can find files, search the project, and use full LSP
navigation without leaving karya. **Shipped 2026-08-07.**

---

## Phase 5 — Skills marketplace ◐
**Goal:** portable SKILL.md packages, installed once, visible to every agent.

- ☑ `internal/skills` — registry client, hash-verified atomic install into the
  karya prefix (Source: HTTP/dir/fake)
- ☑ Default registry + `karya skills registry add <url>`
- ☑ Per-agent materialization (symlinks into detected agents' dirs) + removal on
  uninstall/remove
- ☑ Project-local `.karya/skills/` listed (auto-visible to task agents)
- ☑ `karya skills search|install|remove|list` + `karya doctor` reports skills
- ☐ `internal/skillsview` TUI browser

**Done when:** a skill installs from the registry and is usable by every opted-in
agent in task runs, fully inside karya-owned paths.
**Core shipped 2026-08-07; TUI browser deferred.**

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

---

## Phase 9 — Six-view IDE restructure ◐
**Goal:** reorganize the IDE around six top-level views (workspaces) the user
switches between, with agents as headless backend engines karya drives, a slim
CLI, and seamless cross-view navigation. The views blend together — from any view
you can jump to the right one for a task (git file → editor, task → git/review).

The six views:
1. **Human-in-Control** — editor (nav/search/LSP) + terminal + a read-only
   **Companion** agent pane (asks only, never edits files).
2. **Multi-Agent SE** — dashboard of every task, agent output, and state; create
   and instruct tasks; the configurable role pipeline runs here.
3. **Git** — lazygit-style git ops + worktree list + per-worktree diff.
4. **Review** — open PRs in configured repos + adhoc PR URL; reviewer comments
   over the PR base commit.
5. **Scratch** — ideas/docs/mermaid in a global dir (not the repo), path set in
   Settings.
6. **Settings** — language/LSP/formatter installs; global + project agent
   instructions; agent config; orchestration pipeline; skills; MCP.

**Locked decisions (2026-08-07):** agents are **headless backends karya drives**
(no interactive agent-in-terminal panes for the SE flow); top-level view switcher
(`Ctrl+Space 1-6` + `Ctrl+Space Space` picker); **CLI is just
`version`/`update`/`uninstall`** + bare-`karya` TUI launch; the role pipeline
(looper / knowledge-maintainer / planner / executor / tester-verifier / reviewer)
is **user-configurable** with those six as the default preset.

- ☑ **P1 shell** — view switcher + per-workspace pane trees
  (`internal/ide/workspace.go`), existing panes migrated into their views,
  cross-view jumps (git file `o`→editor, task `g`/`Enter`→git/review)
- ☑ Read-only **Companion** agent pane (`internal/companionview`, headless Q&A)
- ☑ In-process task lifecycle (`internal/tasksvc`) — the TUI no longer shells out
  to `karya <subcommand>`; CLI slimmed (lifecycle commands deleted; other legacy
  tooling commands hidden-but-dispatchable, pending migration into Settings in P8)
- ☐ **P2 Human-in-Control depth** — richer Companion (streaming answers, task/repo
  context via `internal/prompts`), file tree/breadcrumbs, terminal ergonomics
- ☐ **P3 Multi-Agent orchestration** — configurable role pipeline as the default
  preset; per-role headless backend + instructions; per-task per-role status +
  transcripts (build on `internal/agentrun`, `task`, `gate`)
- ☐ **P4 Git worktree depth** — first-class worktree list + per-worktree diff over
  base; jump-to-editor/review from a worktree
- ☐ **P5 Review of PRs** — `gh pr list` for configured repos + adhoc PR URL; fetch
  PR base commit, assemble diff via `internal/diffview`, reviewer emits inline
  comments over the base; optionally post via `gh`
- ☐ **P6 Scratch** — global scratch dir (Settings-configurable), markdown +
  mermaid render, doc/page drafting via a headless backend
- ☐ **P7 Settings UI + MCP** — TUI over `internal/config`/`prefs`/`toolreg`/
  `skills`; global vs project instructions (non-overriding unless opted in);
  orchestration pipeline editor; MCP marketplace (folds in Phase 6)
- ☐ **P8 CLI/tmux cleanup** — remove the hidden legacy subcommands + the
  tmux/session layer once nothing shells out to `karya <subcommand>`

**Architecture & conventions (established in P1 — future phases follow these):**

- **Workspace layer** (`internal/ide/workspace.go`): root `ide.Model` holds
  `workspaces [6]*workspace` + an `active` index. Each `workspace` owns its own
  `*layout.Tree` and its per-view singleton pane IDs (finder/search/git/task/
  review/inbox/editor). `m.tree` is a live pointer to the active workspace's tree,
  kept in sync by `switchTo`/`initWorkspaces`, so all pane code and tests operate
  on one tree. Pane IDs live on `workspace` (not `Model`) because per-tree ids are
  allocated independently and would collide across workspaces.
- **Switching**: `switchTo(kind)` sets `active`, lazily seeds the view's default
  layout on first visit (`seedWorkspace`), and re-syncs pane sizes. Only the
  editor view is seeded eagerly; the rest seed on first switch (fast startup).
  Bound to `Ctrl+Space 1-6`; the picker overlay is `Ctrl+Space Space`.
- **Cross-view navigation = poll-drain request bus** (no channels): a view sets a
  request field (e.g. `gitui.Panel.OpenRequest`, `taskview.Board.GitRequest/
  ReviewRequest`, `companionview.Companion.AskRequest`), and the root model drains
  it in `forward()`, switching to the target workspace first, then acting. Add new
  cross-view intents the same way.
- **Agents are headless**: the Companion (`internal/companionview`) and the task
  lifecycle both call agents via headless one-shot mode off the render path
  (`agent.NewRunner(name).Headless(...)`, `internal/agentrun`). No interactive
  agent PTY panes in the SE flow. Companion answers only — never edits files.
- **In-process lifecycle** (`internal/tasksvc`): `Env{Repo, Store, Worktrees}` +
  `RepoEnv(dir)`; `Plan/Implement/Verify/Merge/NewTask/Start/Abandon/List/
  CrossGate`. The TUI drives the gate lifecycle by calling these directly — it
  never shells out to a `karya` subcommand.
- **Pane migration** (where each existing view lives): editor + terminal +
  Companion, with finder/search, in view 1; task board + gate inbox in view 2;
  git panel (+ worktree list/diff, P4) in view 3; task-gate reviews in view 4
  (PR review is P5); placeholders in views 4–6 until their phases land.

**Risks carried forward (address in P8):** the hidden legacy subcommands
(`shell`, `agent native`, `agent switch-to`, `tui`) are still shelled out by the
tmux/session layer — remove them together with that layer, not before, or a live
session breaks. `syncPaneSizes` sizes only the active tree; keep re-syncing on
`switchTo` when a view was resized while hidden.

**Verification (each phase keeps these green):** `go build ./...`;
`go test ./...`; golangci-lint via `make lint` (runs it through `go run`, not on
PATH); the integration smoke (`go test -tags=integration ./internal/ide/`) launches
`karya tui` on a real PTY. Manual: cycle all six views via `Ctrl+Space 1-6` and
the picker; ask the Companion a question and confirm no file mutation; open a
changed file from Git → lands in the editor; run a task lifecycle op and confirm
it runs in-process (no `karya` subprocess) and the board refreshes.

**Done when:** all six views are usable and blend together, agents run only as
karya-driven backends, and the CLI is just `version`/`update`/`uninstall`.
