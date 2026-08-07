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

**Active phase:** Phase 5 — skills marketplace (next). Phase 0–4 core are shipped:
docs pivot, TUI skeleton, embed Neovim, git panel/task board/review+gates, and
verification/merge (verify+merge CLI, in-TUI approve/reject + gate inbox, agent
PTY panes in worktrees). Phase B (agent adapters) proceeds in parallel (headless);
tdd/cross-agent/regression-net/perf remain within Phase 4.
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

### 2026-08-06 — Phase 1: TUI walking skeleton (shipped)

- New stdlib-only TUI stack, bottom-up, each layer unit-tested:
  `internal/cellbuf` (styled grid + minimal diff + RuneWidth), `internal/term`
  (raw mode via syscall ioctls, ANSI Output, capability detection, pure input
  Decoder), `internal/keymap` (one Ctrl-Space engine, always-intercepted leader,
  which-key), `internal/layout` (tab/pane tree, adjacency focus, resize),
  `internal/tui` (Elm Model/Update/View + Program loop, tested over pipes),
  `internal/pty` + `pty/vt` (PTY host + minimal VT emulator, real-pty tests).
- `internal/ide` composes them into the root model (framed panes, status line,
  which-key popup, shell PTY panes); `karya tui` launches it. Pane creation is
  injectable so the model is unit-tested with fakes; an integration PTY smoke
  test runs the real binary, confirms it renders, and quits on Ctrl-Space Q.
- Completed the carried-over Phase B WIP enough to build: `defaultExec` seam
  restored in `internal/agentrun` (module builds, all tests pass).
- Everything on `feat/single-process-tui-ide`; golangci-lint clean on new pkgs.

### 2026-08-06 — Phase 2: embed Neovim (core)

- `internal/nvimrpc/msgpack`: stdlib MessagePack codec (Marshal + streaming
  Decoder) for the nvim RPC subset (incl. ext handles). Round-trip + fixture tests.
- `internal/nvimrpc`: msgpack-RPC client over `nvim --embed` stdio — request/
  response by msgid, notification dispatch, background reader, concurrent Call,
  Notify, graceful shutdown; UI wrappers (UIAttach/TryResize/Input/Command/
  SetOption). Pure `redraw` → `Grid` reducer (grid_line/scroll/clear/cursor,
  hl_attr_define, default_colors_set, mode_change; wide runes, cell repeat).
  Fake-peer unit tests (-race) + real-nvim integration (typing renders to Grid).
- `internal/ide`: `editorPane` embeds Neovim (grid blit, key forwarding in nvim
  notation, chrome-off, panic-safe redraw signaling); `karya tui <file>` opens a
  file in the embedded editor. PTY smoke: opens a file, content renders through
  RPC+Grid, quits on Ctrl+Space Q.

### 2026-08-06 — Phase 2 completed: engine config + default flip

- `internal/assets/nvimengine/init.lua`: a plugin-free Neovim engine config
  (options, syntax + treesitter, native `vim.lsp.config/enable` for servers on
  PATH, built-in completion on LspAttach). Extracted via `assets.ExtractNvimEngine`
  under the isolated `karya/nvim-engine` app-name (`config.NvimEngineAppName/
  NvimEngineDir`); `editorPane` launches `nvim --embed` with it (falls back to
  `--clean` if extraction fails). Validated by `TestEngineConfigValid`.
- Configurable leader via `KARYA_LEADER` (Ctrl+Space is grabbed by macOS's
  input-source shortcut); `keymap.ParseLeader` + `DefaultBindingsFor`.
- Bare `karya` and `karya edit <file>` now launch the single-process TUI; `karya
  dev` stays the explicit legacy tmux launcher until the Phase 7 removal.

### 2026-08-06 — Zero-setup LSP auto-provisioning

- Opening a file now auto-installs its language server (+ formatter/linter) into
  karya's isolated prefix in the background — no user action, no plugin manager.
  Reuses the existing mise + `toolreg` catalog (hybrid approach; a marketplace
  comes in Phase E/F). `cli.autoProvisioner` (implements `ide.Provisioner`) maps
  language → catalog tool IDs, installs via `Registry.Plan` + `Dispatcher` with
  all output to a log file (never the TUI), deduped/serialized.
- Engine config starts LSP on `FileType` with an executable guard so a lazily-
  installed server attaches via `editorPane.reattachLSP` (`doautocmd FileType`).
- `cmdTUI` builds the app first so the embed inherits karya's managed PATH.
- Deferred: `.karya/project.toml` LSP override (needs a stdlib TOML reader) —
  auto-detect covers the common case.

### 2026-08-07 — Phase 3 core: git panel, task board, review + gates

- `internal/git`: headless git service over a Runner (status/stage/unstage/
  commit/diff/log/branch/push/DiffRange); fake-runner + real-git tests.
- `internal/diffview`: unified-diff parser + cellbuf renderer (colors/scroll),
  shared by the git panel and review.
- `internal/gitui`: built-in git panel (replaces lazygit) — file list, live
  diff, stage/unstage/commit/push; `Ctrl+Space g g/c/p`.
- `internal/taskview`: task board (`Ctrl+Space t t`) over an injected loader.
- `internal/gate` + `internal/review` + `internal/reviewview`: pending-gate
  model, artifact assembly (spec/plan/diff/evidence), scrollable review
  (`Ctrl+Space r`); `karya gate list|approve|reject|delegate` + `karya review`
  record crossings (actor + feedback) in STATE.json.
- IDE: `paneView` interface + `layout.FocusPane`; karya views open as tabs and
  close on `q`/`Esc`.
- Deferred to Phase 4: in-TUI approve/reject keys + gate inbox view, and agent
  CLIs as PTY panes bound to task worktrees.

### 2026-08-07 — Phase 4 core: verification & merge

- `internal/verify`: runs a spec's Verification commands in the task worktree,
  captures exit codes + output, writes `VERIFY-<n>.md` (empty run ≠ pass).
- `karya verify <id>` (numbered evidence) + `karya merge <id>` (git merge or
  `--pr`, post verify-gate only → done, worktree torn down); `git.Repo.Merge`.
- In-TUI gate crossing: reviewview `a` approve / `x` reject-with-feedback via a
  Crosser the Model satisfies; `internal/gateview` inbox (`Ctrl+Space a`, Enter →
  review). Fixed `tree` mise pin + on-launch mise-config resync along the way.
- Agent CLIs as PTY panes bound to a task's worktree from the task board (`a`);
  deterministic agent selection (never the interactive resolver).

### 2026-08-07 — Phase 4.5 (config & continuity) + README refresh

- Default 3-pane layout on bare `karya` (editor + agent + build); layered agent
  instructions global → project → task (`prompts.Context`, opt-in override
  marker) with a global `instructions.md`; per-task `MEMORY.md` (append/read,
  in prompts + review) for agent-swap continuity; a test asserting agents inherit
  karya's managed PATH (IDE tools). DESIGN §5/§6.1/§11 updated.
- README rewritten for the single-process HITL IDE with mermaid diagrams (the
  loop, the gate state machine, the architecture, layered instructions); tasks
  framed as an OKR-shaped contract (Objective + acceptance criteria).

### 2026-08-07 — Phase 5 (skills, partial) + Phase 4.6 (human IDE features)

- Skills marketplace: `internal/skills` (registry Index + HTTP/dir/fake Source,
  hash-verified atomic install, Store list/remove) + `karya skills
  search|install|remove|list|registry`. Per-agent materialization + project
  `.karya/skills/` still pending.
- Human IDE (DESIGN §6.4): full editor LSP navigation (symbols/diagnostics/
  signature help added to the engine config); `internal/finder` fuzzy file
  finder (`Ctrl+Space f`) and `internal/searchview` project live-grep
  (`Ctrl+Space /`), both opening results in the editor via `editorPane.OpenFile`;
  `Ctrl+Space e` focuses the editor. docs/keymaps.md updated + re-vendored.

### 2026-08-07 — Phase 5 complete (skills) + OKF direction

- Skills marketplace done: per-agent materialization (symlinks into detected
  agents' dirs, karya-owned-links only), project `.karya/skills/` listing,
  `karya doctor` skills report, uninstall dematerialize. (TUI browser deferred.)
- Clarified OKF = Google's Open Knowledge Format (markdown + YAML frontmatter,
  files, cross-links, format-is-the-contract). karya's `.karya/` artifacts are
  already OKF-shaped; captured a direction in DESIGN §11 to add OKF frontmatter +
  a `karya knowledge export` for portable, shareable task knowledge.

### 2026-08-07 — In-TUI task lifecycle (Phase B wiring)

- Wired the agent-driving CLI steps: `karya plan <id>` and `karya implement <id>`
  (`internal/cli/plan.go`) wrap `agentrun.RunStep` and own the gate transitions
  (draft→planned on plan; approved→implementing on implement, gated on the plan
  gate), re-running against the latest rejection feedback. Dispatch + usage
  updated.
- The task board is now a keyboard-driven lifecycle surface (`internal/taskview`):
  `n` new (inline slug input → creates the task and opens its spec), `s` start,
  `p` plan, `i` implement, `v` verify, `m` merge — each emits a `LifecycleRequest`
  the IDE fulfils by running the matching `karya` subcommand in the background
  (`Model.runLifecycle` → `lifecycleDoneMsg`), off the render path, then refreshes
  the board in place. Review/approve stays native (`Enter` / `<L> r`).
- `<L> t n` opens the board straight into new-task input; `<L> t s` opens it ready
  to start (both previously "coming soon" stubs). docs/keymaps.md board section
  rewritten + re-vendored.
- Verified: unit tests for board lifecycle keys + input mode and the ide helpers
  (`lifecycleArgs`/`parseCreatedID`/`firstLine`, `<L> t n` flow); `go build`,
  `go vet`, `go test -race ./...`, `-tags=integration` (ide+agentrun), and
  golangci-lint v2 all green.

### 2026-08-07 — Git panel: history, branches, stash + boxed multi-pane UX

- The git panel is now a lazygit-style multi-pane surface instead of a single
  continuous list. Left column stacks four **bordered panes** — Changes,
  Branches, Stashes, Log — with the selected item's diff in a large pane on the
  right, so it's always clear where each section starts and which has focus, and
  the panel is useful even on a clean tree.
- Promoted the IDE's box-frame drawer to a shared primitive `cellbuf.Box`
  (title + focus styling, returns inner rect); `ide.drawFrame` now delegates to
  it — one border implementation for the window manager and views.
- git service (`internal/git`): `Checkout`, `CreateBranch`, `Stash`/`StashList`/
  `StashShow`/`StashPop`; `Commit` carries author + relative date; `Show(ref)`
  for commit diffs.
- Panel keys: `Tab`/`Shift-Tab` cycle panes; `j`/`k` move; `Enter` acts per pane
  (stage · checkout · pop); `s` stash, `b` new-branch input; `c` commit, `P` push
  unchanged. Diff pane previews commit/stash/file diffs. docs/keymaps.md git
  section rewritten + re-vendored.
- Verified: git argv + parse tests; gitui load/focus-cycle/checkout/stash push+pop/
  new-branch/boxed-render tests; `go build`, `go vet`, `-race`, `-tags=integration`
  (ide), golangci-lint v2 all green.

### Resume point (do this next)

1. Phase 6 — MCP marketplace (mirrors skills): `internal/mcp`, per-agent native
   config generation from one source of truth, `karya mcp search|install|sync`.
2. Remaining Phase 4: `tdd:true` acceptance-test-first flow, cross-agent reviewer
   pre-gate filter, auto-detected regression net at the verify gate, perf
   benchmarks vs the §8.4 budgets.
3. Phase B follow-ups (headless): finish the remaining `agentrun` adapters + `Caps`
   matrix and add adapter contract tests via a scripted fake-agent binary
   (`karya plan`/`implement` are now wired end-to-end into the TUI board).
