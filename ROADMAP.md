# karya — Roadmap to v1.0

Phased build order for the human-in-the-loop agent IDE. karya is a **single
self-contained process** that owns the terminal (its own window/pane manager, git
UI, review surfaces, and six top-level views) and embeds Neovim as the
text-editing engine over msgpack-RPC — everything under one unified keymap. Each
phase is shippable and leaves the binary working. Full design:
[DESIGN.md](DESIGN.md). Founding decision:
[ADR 0001](docs/adr/0001-single-process-tui-embed-neovim.md). Live status:
[PROGRESS.md](PROGRESS.md). The v0 roadmap (tmux era, shipped) is in
[archive/v0/ROADMAP.md](archive/v0/ROADMAP.md).

Legend: ☐ not started · ◐ core shipped, follow-ups open · ☑ done

This roadmap is **one linear scheme** in two parts: **Part I — Shipped (P0–P12)**
summarizes the current build; **Part II — Forward (P13–P20)** is the detailed
plan. Older docs and memory used a mixed scheme (Phase A/B, 0–7, and a six-view
"Phase 9"); the mapping to these linear numbers is in
[PROGRESS.md](PROGRESS.md#phase-map-old--new).

---

# Part I — Shipped (the current build)

## Current build at a glance

**What it does now.** karya is a six-view TUI IDE for human-in-the-loop, agents-
first software engineering. You launch it on a repo and switch between six
workspaces with `Ctrl+Space 1-6` (or the `Ctrl+Space Space` picker):

1. **Human-in-Control** — embedded-Neovim editor (LSP/treesitter) + fuzzy finder +
   project search + terminal + a read-only **Companion** agent pane (asks only).
2. **Multi-Agent SE** — task board + gate inbox (dashboard depth is forward work).
3. **Git** — lazygit-style panel (status/stage/commit/diff/log/branch/stash).
4. **Review** — task-gate reviews (PR review is forward work).
5. **Scratch** — placeholder (forward work).
6. **Settings** — placeholder (forward work).

Underneath: a **task engine** — spec contracts, a gate state machine
(draft→planned→approved→implementing→verifying→merging→done) with a `STATE.json`
audit trail, per-task git worktrees, and executable verification; **headless agent
adapters** (claude/codex/crush/gemini/aider/…) behind one interface; **zero-setup**
LSP/toolchains via an isolated mise; a **skills marketplace**; and a **slim CLI**
(`version`/`update`/`uninstall` + bare-`karya` launch) with the lifecycle driven
in-process.

**Package map.**
- *stdlib TUI runtime*: `term` · `cellbuf` · `tui` · `keymap` · `layout` · `pty`(+`pty/vt`)
- *embedded editor*: `nvimrpc`(+`nvimrpc/msgpack`) · `assets/nvimengine`
- *root model + view shell*: `ide` (+ `ide/workspace.go`)
- *views*: `gitui` · `taskview` · `reviewview` · `gateview` · `finder` · `searchview` · `companionview` · `diffview`
- *task/agent engine*: `task` · `spec` · `worktree` · `gate` · `review` · `verify` · `agentrun` · `prompts` · `tasksvc` · `agent`
- *platform*: `config` · `prefs` · `toolreg` · `tools` · `skills` · `assets` · `git` · `cli`

**Conventions established in P12 (future phases follow these):**

- **Workspace layer** (`internal/ide/workspace.go`): the root `ide.Model` holds
  `workspaces [6]*workspace` + an `active` index. Each `workspace` owns its own
  `*layout.Tree` and its per-view singleton pane IDs. `m.tree` is a live pointer to
  the active workspace's tree, kept in sync by `switchTo`/`initWorkspaces`, so all
  pane code and tests operate on one tree. Pane IDs live on `workspace` (not
  `Model`) because per-tree ids are allocated independently and would collide
  across workspaces.
- **Switching**: `switchTo(kind)` sets `active`, lazily seeds the view's default
  layout on first visit (`seedWorkspace`), and re-syncs pane sizes. Only the editor
  view seeds eagerly; the rest seed on first switch (fast startup).
- **Cross-view navigation = poll-drain request bus** (no channels): a view sets a
  request field (e.g. `gitui.Panel.OpenRequest`, `taskview.Board.GitRequest/
  ReviewRequest`, `companionview.Companion.AskRequest`), and the root model drains
  it in `forward()`, switching to the target workspace first, then acting. Add new
  cross-view intents the same way.
- **Agents are headless**: the Companion and the task lifecycle call agents via
  one-shot headless mode off the render path (`agent.NewRunner(name).Headless(...)`,
  `internal/agentrun`). No interactive agent PTY panes in the SE flow.
- **In-process lifecycle** (`internal/tasksvc`): `Env{Repo, Store, Worktrees}` +
  `RepoEnv(dir)`; `Plan/Implement/Verify/Merge/NewTask/Start/Abandon/List/
  CrossGate`. The TUI drives gates by calling these directly — never shells out.

## Shipped phases

Each block is a compact summary; the full chronological detail is in
[PROGRESS.md](PROGRESS.md). `(was …)` cross-references the older scheme.

### P0 — v0 foundation (tmux era, archived) ☑ · *(was v0 Phases 0–8)*
Isolated tmux IDE session, embedded Neovim config, agent-pane management,
six-language scaffolding + toolchains, self-update, install/uninstall, and the
**XDG isolation model**. The presentation layer (tmux/nvim-UI/lazygit) was
replaced by the single-process pivot; the isolation model + toolchain substrate
are reused. Planning docs in `archive/v0/`.

### P1 — Task engine foundation ☑ · *(was Phase A, + 9–14 origins)*
`internal/spec` (SPEC.md contract), `internal/task` (gate state machine +
`STATE.json` audit trail, in-repo `.karya/tasks/<id>/`), `internal/worktree`
(worktree-per-task on `task/<id>`, base-ref + dirty-tree safety), `karya
task`/`karya init`. A task is created from a spec, gets an isolated worktree, and
survives restarts with zero effect on the user's tree.

### P2 — Agent adapter layer (headless) ◐ · *(was Phase B)*
`internal/agentrun` (`Agent` interface, `Caps` matrix, transcripts) + adapters
(claude/codex/crush/gemini/aider/copilot + generic shell), `internal/prompts`
(layered step-prompt assembly), `internal/agent` (`Runner`, `NewRunner`, native
Claude-API engine). Core drives `plan`/`implement` headlessly.
**Follow-ups → P20:** remaining adapter coverage, plan-mode mapping, contract
tests via a scripted fake-agent binary.

### P3 — Single-process pivot: design/docs ◐ · *(was Phase 0)*
DESIGN.md rewritten (single-process runtime, embedded-Neovim engine, unified
keymap, package map, TUI testing strategy) + [ADR 0001](docs/adr/0001-single-process-tui-embed-neovim.md).
**Follow-ups → P20:** AGENTS.md single-process update, `docs/keymaps.md`/
`docs/tutorial.md` rewrite for the six views, docs-drift test.

### P4 — TUI walking skeleton ☑ · *(was Phase 1)*
The stdlib TUI stack: `term` (raw mode, ANSI, input decoder), `cellbuf` (styled
grid + diff), `tui` (Elm Model/Update/View + Program loop), `keymap` (one
`Ctrl-Space` leader engine + which-key), `layout` (tab/pane tree), `pty`(+`vt`)
live shell panes, composed by `internal/ide`. Splittable/focusable/resizable panes
under one keymap; PTY integration smoke test.

### P5 — Embed Neovim as the editing engine ☑ · *(was Phase 2)*
`internal/nvimrpc`(+`msgpack`) drives `nvim --embed` (redraw → `Grid` → `cellbuf`,
input forwarding, chrome-off); `internal/assets/nvimengine` is a plugin-free engine
config (options + treesitter + native LSP + completion) under an isolated app-name;
**zero-setup LSP** auto-installs a file's server/formatter into karya's prefix in
the background. Bare `karya` flips to the TUI. Configurable leader (`KARYA_LEADER`).

### P6 — Git panel, task board, review + gates ☑ · *(was Phase 3)*
`internal/git` (headless service) + `internal/gitui` (lazygit-style multi-pane:
changes/branches/stashes/log + diff), `internal/diffview` (shared unified-diff
renderer), `internal/taskview` (task board), `internal/gate` + `internal/review` +
`internal/reviewview` (pending-gate model, artifact assembly, scrollable review).
The full spec→plan→gate→implement→gate loop with recorded crossings.

### P7 — Verification & merge ◐ · *(was Phase 4)*
`internal/verify` (executable `Verification` in the worktree → `VERIFY-<n>.md`
evidence), merge-or-`--pr` post verify-gate, in-TUI approve/reject +
`internal/gateview` inbox, agent CLIs as PTY panes bound to worktrees.
**Follow-ups → P14/P20:** cross-agent reviewer pre-gate (→P14); `tdd:true`
acceptance-test-first, regression net, perf benchmarks (→P20).

### P8 — Config & continuity ☑ · *(was Phase 4.5)*
Default 3-pane layout on bare `karya`; layered agent instructions global → project
→ task (enhance-not-override, opt-in `<!-- karya:override -->`); per-task
`MEMORY.md` (agent-agnostic, in prompts + review) so agents are swappable
mid-task; agents inherit karya's managed PATH.

### P9 — Human IDE features ☑ · *(was Phase 4.6)*
Full editor LSP navigation (symbols/diagnostics/signature help added to the engine
config); `internal/finder` fuzzy file finder (`Ctrl+Space f`); `internal/searchview`
project live-grep (`Ctrl+Space /`); `editorPane.OpenFile` + focus-editor. A human
can triage, read, and fix without leaving karya.

### P10 — Skills marketplace ◐ · *(was Phase 5)*
`internal/skills` (registry client, hash-verified atomic install into the karya
prefix), default registry + `karya skills registry add`, per-agent materialization
(symlinks into detected agents' dirs), project-local `.karya/skills/`, `karya
skills …` + `karya doctor` report.
**Follow-ups → P18:** `internal/skillsview` TUI browser.

### P11 — In-TUI task lifecycle wiring ☑ · *(was the 2026-08-07 lifecycle work)*
`karya plan`/`karya implement` wrapping `agentrun.RunStep` and owning the gate
transitions; the task board became a keyboard-driven lifecycle surface
(`n`ew/`s`tart/`p`lan/`i`mplement/`v`erify/`m`erge). (Later folded in-process in
P12; the standalone CLI commands were removed.)

### P12 — Six-view shell ☑ · *(was six-view "Phase 9" · P1)*
`internal/ide/workspace.go` (view switcher + per-workspace pane trees), existing
panes migrated into their views, cross-view jumps (git file `o`→editor, task
`g`/`Enter`→git/review), the read-only `internal/companionview` (headless Q&A),
in-process `internal/tasksvc` (no more shelling out), and the **CLI slimmed** to
`version`/`update`/`uninstall` (lifecycle commands deleted; other legacy tooling
commands hidden-but-dispatchable pending P19). Established the conventions above.

---

# Part II — Forward (detailed plan)

Each phase names its goal, the key work, the packages to create or reuse, and its
done-when. Immediate next: **P14** (with **P13** alongside).

## P13 — Human-in-Control depth ☐
**Goal:** the editor view is a full IDE for humans and a genuinely useful
companion.

- **Companion depth:** stream answers incrementally (`agentrun` `Caps.Streaming`)
  instead of a single blocking reply; inject task/repo context via
  `prompts.Context` (global → project → task) so answers are grounded; scrollback +
  copy.
- **File navigation:** a file-tree / project-explorer pane + breadcrumbs + buffer
  list, so browsing doesn't depend on the finder alone.
- **Terminal ergonomics:** multiple terminals, clear/scroll, sensible focus.
- *Reuse:* `companionview`, `prompts`, `agentrun`, `finder`, `editorPane`.

**Done when:** Companion answers stream with task/repo context, and a human can
navigate the project from the editor view without the finder.

## P14 — Multi-Agent orchestration ☐  *(centerpiece)*
**Goal:** multiple named agent roles collaborate to complete a task; the dashboard
lets you observe every task/role/output/state and create/instruct tasks.

- **Role pipeline (default preset):** looper · knowledge-maintainer · planner ·
  executor · tester-verifier · reviewer. Each role maps to a headless backend + a
  role instruction layer. The set and flow are **user-configurable** (the config
  surface is designed here; the full editor lands in P18 Settings).
- **Orchestrator:** sequence roles with hand-offs and the mandatory human gates
  between them — extend `agentrun.RunStep` beyond plan/implement to review/verify
  steps (DESIGN Phases C–D), reusing the `gate` state machine and per-task
  `MEMORY.md` for continuity across roles.
- **Cross-agent review** (implementer ≠ reviewer) as a pre-gate filter — the
  deferred P7 item lands here.
- **Dashboard:** per-task, per-role status + live output/transcripts + current
  state; create tasks; add instructions mid-task (append to `MEMORY.md`).
- *Reuse:* `agentrun`, `task`, `gate`, `prompts`, `tasksvc`, `taskview`; extend
  `taskview` or add `internal/dashboardview`.

**Done when:** a task can be driven through the role pipeline with per-role status
+ transcripts visible, cross-agent review runs before gates, and the human can
create and instruct tasks from the dashboard.

## P15 — Git worktree depth ☐
**Goal:** first-class worktree management in the Git view (agents work on
worktrees, so this is how you inspect their work).

- **Worktree list pane** backed by `worktree.Manager` + `git worktree list`:
  task worktrees with branch, base, and status.
- **Per-worktree diff over its base commit** (`task.Base`) via `git.DiffRange` +
  `diffview`.
- **Jump** from a worktree to the editor/review for that task (cross-view bus).
- *Reuse:* `worktree`, `git` (`DiffRange`/`Show`), `diffview`, `task.Base`.

**Done when:** the Git view lists task worktrees and shows each one's diff over its
base, with jump-to-editor/review.

## P16 — Review of PRs ☐
**Goal:** review open PRs across configured repos, and any adhoc PR URL, with the
reviewer's comments over the PR base.

- **List open PRs:** `gh pr list --json` across the repos configured in Settings.
- **Adhoc URL:** paste a PR URL → `gh pr view` / `gh api` for metadata, base
  commit, and diff.
- **Assemble review:** diff over the PR base via `diffview`; a headless reviewer
  backend emits inline comments over the base; render via the `reviewview` pattern.
  Optionally post via `gh pr review`.
- *Reuse:* `reviewview`, `diffview`, `agentrun`, the `gh` CLI (already used for
  `merge --pr`); new `internal/prreview` + `internal/prview`.

**Done when:** open PRs list, an adhoc PR URL can be reviewed with comments over
its base, and results show in the Review view.

## P17 — Scratch ☐
**Goal:** a global scratchpad for ideas, docs, and mermaid diagrams — stored
outside the repo.

- **Global scratch dir** under `config.Paths` (path is Settings-configurable), not
  the repository.
- **Markdown** editing (reuse `editorPane` on scratch files) + **mermaid**
  render/export; **agent-assisted drafting** (ask a headless backend to draft a doc
  / design a page / produce a diagram).
- *Reuse:* `editorPane`, `config.Paths`, `agentrun`; new `internal/scratchview`.

**Done when:** notes/docs/mermaid live in a global dir (not the repo), are editable
in the Scratch view, and can be drafted with agent help.

## P18 — Settings UI + MCP ☐  *(folds old MCP marketplace)*
**Goal:** configure the whole IDE — global and per-project — from a TUI, and wire
one MCP install into every agent.

- **Settings view** (`internal/settingsview`) over `config`/`prefs`/`toolreg`/
  `tools`/`skills`: language/LSP/formatter installs; **global vs project agent
  instructions** (project does not override global unless the user opts in — reuse
  the `prompts` override marker); agent config; the **P14 orchestration-pipeline
  editor**; the scratch-dir path; a **skills browser** (the deferred P10 item).
- **MCP marketplace:** `internal/mcp` registry + runtime provisioning via the
  isolated mise; `mcp.toml` as the source of truth (global + `.karya/mcp.toml`);
  per-agent **native config renderers** (claude `.mcp.json`, crush `crush.json`,
  gemini `settings.json`, …) regenerated on agent add/remove; permission scoping
  per project; secrets by env-var reference only. Surfaced **in Settings** (no new
  CLI — the CLI stays slim).
- Also fold: **`.karya/project.toml` LSP override** (needs a stdlib TOML reader).
- *Reuse:* `config`, `prefs`, `toolreg`, `tools`, `skills`, `prompts`; new
  `internal/settingsview`, `internal/mcp` (+ MCP browser).

**Done when:** the IDE is fully configurable from the Settings view (global +
project), and installing an MCP server makes it available to all detected agents
without hand-editing any agent config.

## P19 — CLI / tmux teardown ☐
**Goal:** complete the slim-CLI promise and delete the legacy tmux/session layer.

- Remove the **hidden legacy subcommands** (`dev`/`new`/`run`/`edit`/`ship`/`lang`/
  `profile`/`tool`/`install`/`doctor`/`shellenv`/`completion`/`help`/`docs`/
  `tutorial`/`init`/`skills`/`agent`/`tui`/`shell`) once their function lives in
  the views/Settings and nothing shells out to `karya <subcommand>`.
- Delete the **tmux/session layer**: `internal/session`, the tmux client,
  `assets.ExtractTmuxConf`, the agent `Manager`, the `internal/editor` (tmux
  send-to-pane) package, and the hidden `shell`/`agent native`/`agent switch-to`/
  `tui` subprocess callers. Ensure `app.go` bootstrap no longer needs tmux.
- Reconcile tests; relocate any still-needed logic into the views/config.

**Done when:** the CLI is exactly `version`/`update`/`uninstall` + bare-`karya`,
no tmux/session code remains, and build/tests/lint are green.

## P20 — Hardening & v1.0 ☐  *(folds old hardening phase + deferred items)*
**Goal:** production-trustworthy HITL IDE; tag v1.0.

- **Sandbox** (`internal/sandbox`): seatbelt (macOS) / bubblewrap (Linux)
  confinement of agent processes to the task worktree.
- **Registry signing** (cosign) verification for skills + MCP.
- **`karya task audit`** — full gate/delegation history report (surfaced in a
  view).
- **Deferred verification work (from P7):** `tdd:true` acceptance-test-first with a
  failure-signature check; auto-detected per-language regression net at the verify
  gate; performance benchmarks vs the DESIGN §8.4 budgets in CI with checked-in
  baselines.
- **Agent-adapter follow-ups (from P2):** remaining adapters, plan-mode mapping,
  contract tests via a scripted fake-agent binary.
- **OKF:** knowledge frontmatter + `karya knowledge export` (DESIGN §11).
- **Docs completion (from P3):** AGENTS.md single-process update, `docs/keymaps.md`
  + `docs/tutorial.md` for the six views, ADRs for all phases, docs-drift test.
- **Dogfood** karya through karya tasks for one full cycle; **tag v1.0**.

**Done when:** sandboxed execution, signed marketplace content, an audit trail,
perf budgets green, and docs that match behavior. Ship v1.0.

---

## Risks carried forward

- The **hidden legacy subcommands** (`shell`, `agent native`, `agent switch-to`,
  `tui`) are still shelled out by the tmux/session layer — remove them **together
  with that layer in P19**, not before, or a live session breaks.
- `syncPaneSizes` sizes only the active workspace's tree; keep re-syncing on
  `switchTo` when a view was resized while hidden.

## Verification (every phase keeps these green)

`go build ./...` · `go test ./...` · golangci-lint via `make lint` (runs it through
`go run`, not on PATH) · the integration smoke `go test -tags=integration
./internal/ide/` (launches `karya tui` on a real PTY). Manual: cycle all six views
via `Ctrl+Space 1-6` and the picker; ask the Companion a question and confirm no
file mutation; open a changed file from Git → lands in the editor; run a task
lifecycle op and confirm it runs in-process (no `karya` subprocess) and the board
refreshes.
