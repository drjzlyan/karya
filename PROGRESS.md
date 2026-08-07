# karya — Progress Log

Living status document. **Read this first when resuming work.** It records what is
done, what is in flight, and the exact next action. Update it at the end of every
working session.

- **Design:** [DESIGN.md](DESIGN.md)
- **Roadmap:** [ROADMAP.md](ROADMAP.md)
- **Agent/dev guide:** [AGENTS.md](AGENTS.md)
- **v0 history:** [archive/v0/](archive/v0/)

---

## Current status

**Active phase:** **P12 — six-view shell — shipped** (2026-08-07). The IDE is now
organized around six top-level **workspaces** switched with `Ctrl+Space 1-6` (or
the `Ctrl+Space Space` picker): 1 Human-in-Control (editor + terminal + read-only
**Companion** agent pane), 2 Multi-Agent dashboard (task board + gate inbox), 3
Git, 4 Review, 5 Scratch, 6 Settings. Each workspace owns its own pane/tab tree
(`internal/ide/workspace.go`). The Companion (`internal/companionview`) answers via
a headless agent and never edits files. Cross-view jumps work (git file → editor
with `o`; task → git/review with `g`/Enter). The task lifecycle runs **in-process**
via `internal/tasksvc` (no more shelling out to `karya plan <id>`), and the **CLI
is slimmed** to `version`/`update`/`uninstall` + bare-`karya`. Review-of-PRs,
Scratch depth, the Settings UI, and the configurable multi-agent role pipeline are
stubs for later phases.

Everything shipped through P12 is summarized in [ROADMAP.md](ROADMAP.md#part-i--shipped-the-current-build);
the detailed forward plan (P13–P20) is [ROADMAP.md Part II](ROADMAP.md#part-ii--forward-detailed-plan).
ROADMAP + this file are the source of truth (no separate plan doc).

**Next action:** **P14 — Multi-Agent orchestration.** Build the configurable role
pipeline (looper / knowledge-maintainer / planner / executor / tester-verifier /
reviewer) as the default preset, driving headless backends per role via
`internal/agentrun`, extend `agentrun.RunStep` to review/verify steps with the
`gate` state machine, add cross-agent review (implementer ≠ reviewer) as a pre-gate
filter, and surface per-task per-role status + transcripts in the Multi-Agent
dashboard (extend `internal/taskview` or add `internal/dashboardview`). **P13 —
Human-in-Control depth** (streaming Companion answers with task/repo context, file
tree, terminal ergonomics) can land alongside. Keep build / `go test ./...` /
`make lint` / the `-tags=integration` smoke green.

**P12 shipped — key files & tests:**
- New: `internal/ide/workspace.go` (workspace layer, `switchTo`, `seedWorkspace`,
  picker); `internal/companionview/` (headless Q&A pane); `internal/tasksvc/`
  (in-process lifecycle + `tasksvc_test.go`).
- `internal/ide/ide.go` — `Model` now holds `workspaces [6]*workspace` + `active` +
  a live `tree` pointer; `runLifecycle` calls `tasksvc` (dropped the `os/exec`
  subprocess + `lifecycleArgs`/`parseCreatedID`); `crossGate` delegates to
  `tasksvc.CrossGate`; new `askCompanion`/`companionAnswerMsg`,
  `openFileInEditorView`, `openGitForTask`; `forward()` drains the new requests.
- `internal/keymap/bindings.go` — `ActionView*`/`ActionViewPicker`; digits `1-6`
  repurposed to view switches (removed `ActionTabGoto*`).
- `internal/gitui/gitui.go` — `o` → `OpenRequest()` (open file in editor).
- `internal/taskview/taskview.go` — `g` → `GitRequest()` (jump to Git view).
- `internal/cli/cli.go` — dispatch/usage trimmed to `version`/`update`/`uninstall`
  + bare TUI; **deleted** `plan.go`/`verifymerge.go`/`task.go`/`gate.go` (+ their
  tests) since the lifecycle logic moved to `tasksvc` and the `unused` linter
  forbids dead code. Other legacy tooling commands stay dispatchable-but-hidden
  (removed with the tmux/session layer in P19).
- Tests: `internal/ide/{ide,lifecycle}_test.go`, `keymap/keymap_test.go`;
  gate/verify coverage moved into `tasksvc_test.go`. Full `go test ./...`,
  `go vet ./...`, golangci-lint, and the integration smoke all pass.

---

## Phase map (old → new)

The roadmap was renumbered into one linear scheme (P0–P20). Historical entries
below use the older labels; this table reconciles them.

| New | Title | Old label | Status |
|-----|-------|-----------|--------|
| P0  | v0 tmux IDE + isolation/toolchain substrate (archived) | v0 Phases 0–8 | ☑ |
| P1  | Task engine foundation (spec, gates, worktree, `karya task`/`init`) | Phase A (+ 9–14 origins) | ☑ |
| P2  | Agent adapter layer (headless) | Phase B | ◐ |
| P3  | Single-process pivot — design/docs (DESIGN/ROADMAP, ADR 0001) | Phase 0 | ◐ |
| P4  | TUI walking skeleton | Phase 1 | ☑ |
| P5  | Embed Neovim engine (+ zero-setup LSP) | Phase 2 | ☑ |
| P6  | Git panel, task board, review + gates | Phase 3 | ☑ |
| P7  | Verification & merge | Phase 4 | ◐ |
| P8  | Config & continuity | Phase 4.5 | ☑ |
| P9  | Human IDE features (LSP nav, finder, search) | Phase 4.6 | ☑ |
| P10 | Skills marketplace | Phase 5 | ◐ |
| P11 | In-TUI task lifecycle wiring | (2026-08-07 work) | ☑ |
| P12 | Six-view shell (switcher, companion, tasksvc, slim CLI) | six-view "Phase 9" · P1 | ☑ |
| P13 | Human-in-Control depth | six-view · P2 | ☐ |
| P14 | Multi-Agent orchestration | six-view · P3 | ☐ |
| P15 | Git worktree depth | six-view · P4 | ☐ |
| P16 | Review of PRs | six-view · P5 | ☐ |
| P17 | Scratch | six-view · P6 | ☐ |
| P18 | Settings UI + MCP (folds old Phase 6) | six-view · P7 + Phase 6 | ☐ |
| P19 | CLI/tmux teardown | six-view · P8 | ☐ |
| P20 | Hardening & v1.0 (folds old Phase 7 + deferrals) | Phase 7 (+ deferrals) | ☐ |

The memory notes still say "six-view Phase 9 P1–P8"; that maps to **P12–P19** here.

---

## Pivots (direction changes, and what each kept)

karya has re-aimed three times. Each pivot kept the layers below the presentation
tier and replaced the tier above.

1. **v0 — AI-first tmux IDE (shipped, archived).** An isolated tmux session +
   standalone Neovim UI + lazygit, one BYO agent CLI in a pane, six-language
   scaffolding + toolchains, self-update, install/uninstall. **Kept:** the XDG
   isolation model and the mise/toolchain substrate. **Replaced:** the tmux /
   nvim-UI / lazygit presentation layer.

2. **2026-08-05 — v1.0 replan: human-in-the-loop, agents-first.** Product reset
   from "AI-first with an agent pane" to tasks with **spec contracts**, isolated
   **git worktrees**, mandatory **human gates** (plan / diff / verification),
   agents behind one adapter layer, and skills + MCP marketplaces. A new DESIGN.md
   was written; old planning docs archived to `archive/v0/`.

3. **The superseded "9–14" arc (code kept & adapted).** An earlier agents-first
   build already on `main` (pluggable `agent.Runner`, a first `internal/task` +
   `internal/worktree` on `karya/<id>` branches under the karya prefix, a native
   Claude-API engine, a fleet dashboard). **Adapted:** `Runner` became P2's adapter
   seam; task/worktree moved to in-repo `.karya/tasks/<id>/` on `task/<id>`
   branches with spec contracts + `STATE.json` (P1); the native engine plugs in
   behind `NewRunner`.

4. **2026-08-06 — single-process pivot.** karya stopped orchestrating tmux + a
   standalone Neovim UI + lazygit (three keymaps) and became **one process that
   draws its own screen**, embedding Neovim as the editing engine over msgpack-RPC
   under one leader. Two locked decisions: embed Neovim via `nvim --embed`; stdlib
   only (own cell buffer, terminal I/O, PTY host, msgpack-RPC). See
   [ADR 0001](docs/adr/0001-single-process-tui-embed-neovim.md). **Kept:** the task
   engine (P1) and agent adapters (P2) unchanged.

5. **2026-08-07 — six-view restructure.** The IDE reorganized around six top-level
   views the user switches between; agents became **headless backends karya
   drives** (no interactive agent-in-terminal panes for the SE flow); the CLI
   slimmed to `version`/`update`/`uninstall` with the lifecycle run in-process; the
   multi-agent role pipeline is user-configurable with a six-role default preset.
   P12 (the shell) shipped; P13–P19 fill the views.

---

## Chronological log

### 2026-08-05 — v1.0 replan: human-in-the-loop agent IDE

- Product direction reset: from "AI-first terminal IDE with an agent pane" to
  **human-in-the-loop, agent-based IDE** — tasks with spec contracts, isolated git
  worktrees, mandatory human gates (plan / diff / verification), blended agent CLIs
  behind one adapter layer, skills + MCP marketplaces, and a karya-native modal TUI
  (Neovim-inspired keymaps, tested like a backend).
- New [DESIGN.md](DESIGN.md) written: task state machine, spec format (objective +
  executable acceptance criteria), two-level isolation, agent adapter interface,
  review UX, TUI testing strategy (model/snapshot/PTY/e2e), performance budget,
  marketplace trust model, migration from v0.
- Old planning docs archived to `archive/v0/` (PLAN, ROADMAP, PROGRESS,
  TOOLING_REFACTOR ×2); ROADMAP.md and PROGRESS.md reset for v1.0. AGENTS.md merged
  (single file; old AGENT.md removed) and repointed at DESIGN.md.

### 2026-08-06 — P1 (was Phase A): task engine foundation

- **`internal/spec`** (new) — hand-parsed, stdlib-only SPEC.md format per DESIGN.md
  §3: front-matter (id/status/agent/per-step pins/tdd) + Objective / Acceptance
  criteria (checkboxes) / Context / Constraints / Verification (`cmd:` list).
  Parse/Validate/Render (canonical, round-trip stable) + Template. Hermetic tests.
- **`internal/task`** (reworked) — tasks moved from the 9–14-era karya-prefix JSON
  store to in-repo `.karya/tasks/<id>/` (SPEC.md + STATE.json). Gate state machine
  per DESIGN.md §2 (draft→planned→approved→implementing→verifying→merging→done,
  rejections loop back with mandatory feedback, every crossing records
  actor/gate/timestamp). `EnsureProjectDir` installs a `.karya/.gitignore` that
  keeps runtime state local but SPEC.md committable (verified against real git).
- **`internal/worktree`** (reworked) — branch prefix `karya/` → `task/`; `AddFrom`
  adds base-ref selection (dirty-tree safety: uncommitted changes never leak into a
  task). `Remove` prunes the empty per-project dir. Integration test proves base-ref
  + dirty-tree containment with real git.
- **CLI reshaped** — `karya task new|list|status|show|start|abandon`; `karya init`
  scaffolds `.karya/` + a toolchain-detected repo AGENTS.md. Removed the 9–14
  surface (dashboard/switch/plan/approve-plan/review/merge/reject/checkpoint/
  rewind/allow) — agent-driven steps return in later phases. `taskContext` resolves
  the caller's repo first (a pane cd'd outside the project no longer scatters
  tasks). (Shipped on `main` via #58.)

### 2026-08-06 — P3 (was Phase 0): single-process pivot (design)

- **Direction change (user-approved):** karya stops orchestrating tmux + a
  standalone Neovim UI + lazygit (each with its own keymap) and becomes a **single
  process** that draws its own screen: its own window/pane/tab manager, git panel,
  task/gate/review views, and PTY-hosted shells/agents — with Neovim **embedded as
  the editing engine** over msgpack-RPC. One leader (`Ctrl-Space`), one keymap.
- **Two locked decisions:** (1) embed Neovim via `nvim --embed` RPC rather than
  building an editor from scratch; (2) stdlib-only — build the cell buffer + diff
  renderer, ANSI/terminfo terminal I/O, PTY host, and msgpack-RPC client ourselves.
- **Docs rewritten:** DESIGN.md (single-process runtime, unified keymap,
  embedded-Neovim engine, package map, removed packages, four-level TUI testing),
  ROADMAP.md, PROGRESS.md, ADR 0001; AGENTS.md + docs/keymaps.md + docs/tutorial.md
  follow-ups tracked (→ P20).
- **Branch:** work continues on `feat/single-process-tui-ide` (from `main`).

### 2026-08-06 — P4 (was Phase 1): TUI walking skeleton (shipped)

- New stdlib-only TUI stack, bottom-up, each layer unit-tested: `internal/cellbuf`
  (styled grid + minimal diff + RuneWidth), `internal/term` (raw mode via syscall
  ioctls, ANSI Output, capability detection, pure input Decoder), `internal/keymap`
  (one Ctrl-Space engine, always-intercepted leader, which-key), `internal/layout`
  (tab/pane tree, adjacency focus, resize), `internal/tui` (Elm Model/Update/View +
  Program loop, tested over pipes), `internal/pty` + `pty/vt` (PTY host + minimal VT
  emulator, real-pty tests).
- `internal/ide` composes them into the root model (framed panes, status line,
  which-key popup, shell PTY panes); `karya tui` launches it. Pane creation is
  injectable so the model is unit-tested with fakes; an integration PTY smoke test
  runs the real binary, confirms it renders, and quits on Ctrl-Space Q.
- Completed the carried-over P2 WIP enough to build: `defaultExec` seam restored in
  `internal/agentrun`. golangci-lint clean on new pkgs.

### 2026-08-06 — P5 (was Phase 2): embed Neovim (core)

- `internal/nvimrpc/msgpack`: stdlib MessagePack codec (Marshal + streaming Decoder)
  for the nvim RPC subset (incl. ext handles). Round-trip + fixture tests.
- `internal/nvimrpc`: msgpack-RPC client over `nvim --embed` stdio — request/
  response by msgid, notification dispatch, background reader, concurrent Call,
  Notify, graceful shutdown; UI wrappers (UIAttach/TryResize/Input/Command/
  SetOption). Pure `redraw` → `Grid` reducer (grid_line/scroll/clear/cursor,
  hl_attr_define, default_colors_set, mode_change; wide runes, cell repeat).
  Fake-peer unit tests (-race) + real-nvim integration (typing renders to Grid).
- `internal/ide`: `editorPane` embeds Neovim (grid blit, key forwarding in nvim
  notation, chrome-off, panic-safe redraw signaling); `karya tui <file>` opens a
  file in the embedded editor. PTY smoke: opens a file, renders through RPC+Grid,
  quits on Ctrl+Space Q.

### 2026-08-06 — P5 completed: engine config + default flip

- `internal/assets/nvimengine/init.lua`: a plugin-free Neovim engine config
  (options, syntax + treesitter, native `vim.lsp.config/enable` for servers on PATH,
  built-in completion on LspAttach). Extracted via `assets.ExtractNvimEngine` under
  the isolated `karya/nvim-engine` app-name; `editorPane` launches `nvim --embed`
  with it (falls back to `--clean` if extraction fails). `TestEngineConfigValid`.
- Configurable leader via `KARYA_LEADER` (Ctrl+Space is grabbed by macOS's
  input-source shortcut); `keymap.ParseLeader` + `DefaultBindingsFor`.
- Bare `karya` and `karya edit <file>` now launch the single-process TUI; `karya
  dev` stays the explicit legacy tmux launcher until the P19 removal.

### 2026-08-06 — Zero-setup LSP auto-provisioning (P5)

- Opening a file now auto-installs its language server (+ formatter/linter) into
  karya's isolated prefix in the background — no user action, no plugin manager.
  Reuses the existing mise + `toolreg` catalog. `cli.autoProvisioner` (implements
  `ide.Provisioner`) maps language → catalog tool IDs, installs via `Registry.Plan`
  + `Dispatcher` with all output to a log file (never the TUI), deduped/serialized.
- Engine config starts LSP on `FileType` with an executable guard so a
  lazily-installed server attaches via `editorPane.reattachLSP` (`doautocmd
  FileType`). `cmdTUI` builds the app first so the embed inherits karya's managed
  PATH. Deferred: `.karya/project.toml` LSP override (→ P18).

### 2026-08-07 — P6 (was Phase 3) core: git panel, task board, review + gates

- `internal/git`: headless git service over a Runner (status/stage/unstage/commit/
  diff/log/branch/push/DiffRange); fake-runner + real-git tests.
- `internal/diffview`: unified-diff parser + cellbuf renderer (colors/scroll),
  shared by the git panel and review.
- `internal/gitui`: built-in git panel (replaces lazygit) — file list, live diff,
  stage/unstage/commit/push; `Ctrl+Space g g/c/p`.
- `internal/taskview`: task board (`Ctrl+Space t t`) over an injected loader.
- `internal/gate` + `internal/review` + `internal/reviewview`: pending-gate model,
  artifact assembly (spec/plan/diff/evidence), scrollable review (`Ctrl+Space r`);
  `karya gate list|approve|reject|delegate` + `karya review` record crossings
  (actor + feedback) in STATE.json.
- IDE: `paneView` interface + `layout.FocusPane`; karya views open as tabs and
  close on `q`/`Esc`. In-TUI approve/reject + agent panes deferred to P7.

### 2026-08-07 — P7 (was Phase 4) core: verification & merge

- `internal/verify`: runs a spec's Verification commands in the task worktree,
  captures exit codes + output, writes `VERIFY-<n>.md` (empty run ≠ pass).
- `karya verify <id>` (numbered evidence) + `karya merge <id>` (git merge or
  `--pr`, post verify-gate only → done, worktree torn down); `git.Repo.Merge`.
- In-TUI gate crossing: reviewview `a` approve / `x` reject-with-feedback via a
  Crosser the Model satisfies; `internal/gateview` inbox (`Ctrl+Space a`, Enter →
  review). Fixed `tree` mise pin + on-launch mise-config resync along the way.
- Agent CLIs as PTY panes bound to a task's worktree from the task board (`a`);
  deterministic agent selection. (Interactive agent panes retired in P12 for the
  SE flow.)

### 2026-08-07 — P8 (was Phase 4.5): config & continuity + README refresh

- Default 3-pane layout on bare `karya` (editor + agent + build); layered agent
  instructions global → project → task (`prompts.Context`, opt-in override marker)
  with a global `instructions.md`; per-task `MEMORY.md` (append/read, in prompts +
  review) for agent-swap continuity; a test asserting agents inherit karya's
  managed PATH. DESIGN §5/§6.1/§11 updated.
- README rewritten for the single-process HITL IDE with mermaid diagrams (the loop,
  the gate state machine, the architecture, layered instructions).

### 2026-08-07 — P10 (was Phase 5, partial) + P9 (was Phase 4.6): human IDE features

- Skills marketplace: `internal/skills` (registry Index + HTTP/dir/fake Source,
  hash-verified atomic install, Store list/remove) + `karya skills
  search|install|remove|list|registry`.
- Human IDE (DESIGN §6.4): full editor LSP navigation (symbols/diagnostics/
  signature help added to the engine config); `internal/finder` fuzzy file finder
  (`Ctrl+Space f`) and `internal/searchview` project live-grep (`Ctrl+Space /`),
  both opening results in the editor via `editorPane.OpenFile`; `Ctrl+Space e`
  focuses the editor.

### 2026-08-07 — P10 complete (skills) + OKF direction

- Skills marketplace done: per-agent materialization (symlinks into detected
  agents' dirs, karya-owned-links only), project `.karya/skills/` listing, `karya
  doctor` skills report, uninstall dematerialize. (TUI browser → P18.)
- Clarified OKF = Google's Open Knowledge Format (markdown + YAML frontmatter,
  files, cross-links). karya's `.karya/` artifacts are already OKF-shaped; captured
  a direction in DESIGN §11 to add OKF frontmatter + a `karya knowledge export`
  (→ P20).

### 2026-08-07 — P11: in-TUI task lifecycle wiring

- Wired the agent-driving CLI steps: `karya plan <id>` and `karya implement <id>`
  (`internal/cli/plan.go`) wrap `agentrun.RunStep` and own the gate transitions
  (draft→planned on plan; approved→implementing on implement, gated), re-running
  against the latest rejection feedback.
- The task board became a keyboard-driven lifecycle surface (`internal/taskview`):
  `n` new, `s` start, `p` plan, `i` implement, `v` verify, `m` merge — each emitted
  a `LifecycleRequest` the IDE fulfilled by running the matching `karya` subcommand
  in the background, then refreshing the board. (P12 moved this in-process.)
- Verified: board lifecycle-key + input-mode tests, ide helpers; `go build`, `go
  vet`, `go test -race ./...`, `-tags=integration`, golangci-lint v2 all green.

### 2026-08-07 — Git panel: history, branches, stash + boxed multi-pane UX (P6)

- The git panel became a lazygit-style multi-pane surface: a left column of four
  **bordered panes** — Changes, Branches, Stashes, Log — with the selected item's
  diff on the right, useful even on a clean tree.
- Promoted the IDE's box-frame drawer to a shared primitive `cellbuf.Box`;
  `ide.drawFrame` delegates to it — one border implementation for the window
  manager and views.
- git service: `Checkout`, `CreateBranch`, `Stash`/`StashList`/`StashShow`/
  `StashPop`; `Commit` carries author + relative date; `Show(ref)` for commit diffs.
- Panel keys: `Tab`/`Shift-Tab` cycle panes; `j`/`k` move; `Enter` acts per pane
  (stage · checkout · pop); `s` stash, `b` new-branch, `c` commit, `P` push.
- Verified: git argv + parse tests; gitui load/focus-cycle/checkout/stash/branch/
  render tests; `go build`, `go vet`, `-race`, `-tags=integration`, golangci-lint.

### 2026-08-07 — P12 (six-view shell): workspaces + companion + in-process lifecycle + slim CLI

- Reorganized the IDE around six top-level workspaces (`internal/ide/workspace.go`)
  switched with `Ctrl+Space 1-6` + a picker; each owns its own pane/tab tree with
  per-workspace pane IDs; `m.tree` is a live pointer to the active tree.
- New read-only `internal/companionview` (headless Q&A, never edits files) replaces
  the interactive agent pane in the editor view.
- New `internal/tasksvc` (`Env`/`RepoEnv` + Plan/Implement/Verify/Merge/NewTask/
  Start/Abandon/List/CrossGate) runs the lifecycle **in-process**; `runLifecycle`
  no longer shells out; `crossGate` delegates to it.
- Cross-view request bus: `gitui` `o`→editor, `taskview` `g`/`Enter`→git/review,
  companion questions → headless agent.
- keymap: digits `1-6` → view switches (removed `ActionTabGoto*`).
- CLI slimmed to `version`/`update`/`uninstall` + bare TUI; deleted the superseded
  lifecycle commands (`plan`/`verifymerge`/`task`/`gate` + tests); other legacy
  tooling commands hidden-but-dispatchable pending P19. gate/verify coverage moved
  to `tasksvc_test.go`.
- Verified: `go build ./...`, `go test ./...`, `go vet`, golangci-lint,
  `-tags=integration` smoke all green. Committed on `feat/single-process-tui-ide`.

---

## Resume point (do this next)

1. **P14 — Multi-Agent orchestration** (centerpiece): the configurable role
   pipeline + orchestrator over `agentrun`/`gate`, cross-agent review pre-gate, and
   the dashboard. **P13 — Human-in-Control depth** can land alongside.
2. Everything else is sequenced in [ROADMAP.md Part II](ROADMAP.md#part-ii--forward-detailed-plan)
   (P15 worktree depth · P16 PR review · P17 scratch · P18 settings + MCP · P19
   CLI/tmux teardown · P20 hardening & v1.0). Deferred items from `◐` phases are
   reassigned there: MCP → P18; skills TUI browser → P18; `.karya/project.toml` LSP
   override → P18; tdd/regression/perf → P20; cross-agent reviewer → P14; agentrun
   adapter follow-ups → P20; docs (AGENTS/keymaps/tutorial/drift) → P20.
