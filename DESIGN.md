# karya — v1.0 Design Plan: The Human-in-the-Loop Agent IDE

> **karya** (कार्य — "work/task") is a **human-in-the-loop, agent-based IDE**
> delivered as a single self-contained Go binary. Humans set intent and review
> at explicit gates; coding agents plan, implement, and verify inside isolated
> task environments. karya is **one process**: it owns the terminal — its own
> window/pane manager, git UI, and task/review surfaces — and embeds Neovim as
> the text-editing engine over msgpack-RPC. One consistent, keyboard-first
> workflow under a single leader key — **without touching any of the user's
> existing settings**.

This document supersedes the v0 design (`archive/v0/PLAN.md`) and the earlier
orchestrator-era v1.0 design. v0 shipped system-level isolation, embedded
Neovim/tmux configs, agent detection, language tooling, and self-update. v1.0
keeps the **headless workflow engine** (tasks, specs, worktrees, agent adapters)
and **replaces the presentation layer**: instead of orchestrating an external
tmux + a standalone Neovim UI + lazygit — each with its own keymaps — karya is
now a single-process TUI that owns everything the user sees. Track execution in
[ROADMAP.md](ROADMAP.md) and [PROGRESS.md](PROGRESS.md); engineering rules in
[AGENTS.md](AGENTS.md); the founding decision in
[docs/adr/0001-single-process-tui-embed-neovim.md](docs/adr/0001-single-process-tui-embed-neovim.md).

---

## 1. Product vision

Coding agents today are chat panes bolted onto editors. The human copy-pastes
intent in, eyeballs a wall of diff, and hopes the tests that ran were the right
ones. The loop is implicit, the artifacts (plan, diff, test evidence) are
ephemeral, and every agent CLI speaks a different dialect. And the IDE itself is
a bundle of separate tools — a multiplexer, an editor, a git UI — each with its
own leader key and keymap grammar, so muscle memory never transfers.

karya v1.0 makes the loop explicit and first-class, inside one coherent program:

```
HUMAN                    karya                       AGENT
  │                        │                            │
  │── write/refine SPEC ──▶│                            │
  │                        │──── spawn task (worktree)─▶│
  │                        │                            │── produces PLAN
  │◀─── review PLAN ───────│◀───────────────────────────│
  │── approve / send back ─▶│                            │
  │                        │                            │── implements
  │◀─── review DIFF ───────│◀───────────────────────────│
  │── approve / send back ─▶│                            │
  │                        │                            │── runs VERIFICATION
  │◀─── review EVIDENCE ───│◀───────────────────────────│
  │── approve ────────────▶│── merge / ship ───────────▶│
```

Everything the agent produces — plan, diff, test evidence — is a **reviewable
artifact** stored on disk. Nothing merges without a human gate crossing. Any
agent CLI can drive any step, through one adapter layer.

Crucially, karya is **not only an agent surface**: when an agent gets stuck or a
human needs to triage an issue, read code closely, or make a change by hand, the
same TUI is a **full-featured editor** — fuzzy file navigation, project-wide
search, and complete LSP navigation (definition, references, symbols,
diagnostics, rename) are first-class, so the human is never forced out to another
tool (§6.4).

### Design pillars

1. **The Task is the unit of work.** Not a chat session, not a branch — a task:
   a spec, an isolated worktree, an agent, artifacts, and a gate history.
2. **Human gates are mandatory, not advisory.** Plan, implementation, and
   verification each require an explicit human approval to advance. The human
   can delegate a gate to an agent, but the delegation is recorded.
3. **Agent-agnostic.** claude, codex, crush, gemini, aider, copilot, and future
   agents are interchangeable behind one adapter interface — including mixing
   agents across steps (plan with one, implement with another).
4. **Isolation at two levels.** System-level (never touch user dotfiles,
   Homebrew, global mise) and task-level (git worktree + branch per task;
   agent changes are physically incapable of landing on your working tree
   until you merge them).
5. **Artifacts over chat logs.** Specs, plans, diffs, and verification reports
   are Markdown/JSON files in the repo, diffable and greppable by humans *and*
   re-ingestible by agents.
6. **One process owns the terminal.** karya is a single binary that draws its
   own screen: its own window/pane/tab manager (no tmux), its own git panel (no
   lazygit), its own task/gate/review views, and PTY-hosted shells and agent
   CLIs. The editor is **Neovim embedded as a headless engine** over msgpack-RPC
   — karya renders Neovim's grid into its own cell buffer. No Electron, no
   browser, no external multiplexer. Startup and gate operations feel instant
   (§7.4 performance budget).
7. **One leader, one keymap grammar.** karya intercepts every keystroke and
   dispatches it through a single unified keymap engine: one leader key
   (`Ctrl-Space`) drives pane focus/nav, resize, splits, tabs, git, tasks, and
   gates — the *same* chords whether the focus is the editor, a shell, or an
   agent pane. Which-key discovery is built in. Keys karya does not claim are
   forwarded to the focused pane (into embedded Neovim, or a PTY). There is no
   longer a separate tmux prefix, a separate Neovim leader, and a separate
   lazygit keymap — there is one grammar (§6.1, §6.2).
8. **Reuse the right amount, build the rest from scratch.** Neovim (as an
   embedded engine), git, and agent CLIs do work that is genuinely hard to
   redo — reuse them. Everything the user *interacts with* — windowing, the git
   UI, keymaps, review surfaces — karya builds itself so it is consistent and
   testable. Single static binary, **stdlib only**, no CGO (§6.3, AGENTS.md).

---

## 2. Core concepts

| Concept | Definition | On-disk home |
|---|---|---|
| **Task** | A unit of work moving through the gate state machine | `.karya/tasks/<id>/` (repo) |
| **Spec** | The contract for a task: objective, acceptance criteria, verification, constraints | `.karya/tasks/<id>/SPEC.md` |
| **Plan** | The agent's proposed implementation approach | `.karya/tasks/<id>/PLAN.md` |
| **Worktree** | Isolated git worktree + branch where the agent works | `~/.local/share/karya/worktrees/<project>/<id>` |
| **Artifact** | Any reviewable output: plan, diff snapshot, verification report | `.karya/tasks/<id>/` |
| **Gate** | A human (or delegated) approval point between states | recorded in `.karya/tasks/<id>/STATE.json` |
| **Agent adapter** | Normalized driver for one agent CLI | `internal/agentrun/` |
| **Pane** | A rectangle in the karya UI hosting the editor, a PTY (shell/agent), or a karya view | `internal/layout/` |
| **Skill** | A portable capability package (`SKILL.md` + assets) for agents | installed under karya prefix |
| **MCP server** | A tool/context server usable by agents | installed + configured by karya |

### The task state machine

```
DRAFT ──▶ PLANNED ──gate:plan──▶ APPROVED ──▶ IMPLEMENTING ──gate:diff──▶
VERIFYING ──gate:verify──▶ MERGING ──▶ DONE
   ▲──────────┘  (reject sends back with feedback)  ▲──────────┘
```

- Every transition is recorded in `STATE.json` (who/what approved, when, with
  what feedback). The history is the audit trail.
- Rejection at any gate loops the task back to the agent **with the human's
  feedback appended to the task context** — the agent revises, the human
  re-reviews. This is the core HITL loop.
- `karya task list` shows every task and its state; the task board view shows it
  live inside the IDE.

---

## 3. The Spec format (the human↔agent contract)

The spec is the single document both audiences optimize for. It replaces
scattered prompt history with a durable, versioned contract. Format
(`.karya/tasks/<id>/SPEC.md`):

```markdown
---
id: 2026-08-05-add-retry-to-downloader
status: approved
agent: claude            # preferred agent for implementation (optional)
---

## Objective
One paragraph. What outcome, why it matters. (The "O" — qualitative.)

## Acceptance criteria        ← machine-checkable "key results"
- [ ] `download` retries transient 5xx up to 3 times with backoff
- [ ] permanent failures (4xx, checksum mismatch) never retry
- [ ] `go test ./internal/tools/ -run Retry` passes

## Context
Files/packages the agent must read first; links to relevant docs; what is
explicitly out of scope.

## Constraints
No new dependencies. Must not change the checksum verification order.

## Verification
- cmd: go test -race ./internal/tools/...
- cmd: make lint
```

Why this shape:

- **Objective + acceptance criteria** borrow the OKR split (qualitative intent +
  measurable results) but scoped to a task, where "key results" are literally
  checkable by running commands. Humans write intent; agents get unambiguous
  done-ness; karya can *execute* the criteria.
- **The `Verification` block is executable.** karya runs those commands itself
  at the verify gate and records exit codes + output as the evidence artifact.
  The agent cannot self-certify; karya certifies.
- **Checkboxes in acceptance criteria are checkable by a reviewer agent** (a
  different agent than the implementer, when available) as a pre-human filter.

`karya task new` scaffolds this from a template; `karya task refine` sends a
draft spec through an agent to sharpen acceptance criteria before the human
approves.

---

## 4. Task-level isolation

System-level isolation (unchanged in spirit): everything under karya-owned XDG
dirs, `NVIM_APPNAME=karya/nvim`, opt-in shellenv, vendored mise. See
`archive/v0/PLAN.md` §2 — it still governs, **with one change from the pivot**:
karya no longer runs a dedicated tmux server. It hosts shells and agent CLIs in
its own PTY panes (`internal/pty`), each spawned through the `karya shell`
wrapper so the isolated environment and prompt are preserved without touching the
user's rc files.

Task-level isolation (v1.0):

- **One git worktree per task.** `karya task start` creates
  `git worktree add <karya-prefix>/worktrees/<project>/<id> -b task/<id>`.
  The agent's PTY pane(s) and any headless runs are locked to that worktree
  (cwd + `GIT_DIR`/`GIT_WORK_TREE` set). The user's working tree is never the
  agent's working tree.
- **Agents cannot merge.** Only the verify gate crossing runs `git merge` (or
  `gh pr create` in PR mode), executed by karya, not the agent.
- **Clean teardown.** `karya task abandon` removes worktree + branch +
  artifacts; `karya task done` keeps artifacts, removes worktree.
- **Dirty-tree safety.** Tasks branch from a chosen base ref (default: current
  `HEAD`); uncommitted changes in the user's tree never leak into a task.
- **Defense in depth (later phases):** optional OS-level sandboxing of agent
  processes (seatbelt on macOS, bubblewrap on Linux) restricting writes to the
  task worktree; network policy per task. Designed behind an interface from day
  one, implemented incrementally.

---

## 5. The agent adapter layer (blending every CLI)

Each agent CLI is different: launch flags, headless mode, plan-mode support,
config file locations, MCP config format, skill support, session resume.
`internal/agentrun` normalizes them:

```go
// Agent is the normalized interface every coding-agent CLI implements.
type Agent interface {
    Name() string
    Caps() Caps          // PlanMode, Headless, Resume, Skills, MCP, Streaming
    Plan(ctx, t Task) (PlanResult, error)      // produce PLAN.md
    Implement(ctx, t Task) (RunResult, error)  // work in the task worktree
    Review(ctx, t Task, target Artifact) (ReviewResult, error)
    Verify(ctx, t Task) (RunResult, error)     // run/fix toward verification
}
```

- **Adapters:** `claude`, `codex`, `crush`, `gemini`, `aider`, `copilot`, plus a
  generic `shell` adapter (any CLI that reads a prompt file and edits a cwd) so
  unsupported agents still work in degraded mode.
- **Headless-first.** Adapters drive agents non-interactively
  (`claude -p`, `codex exec`, `crush run`, …) with transcripts captured to the
  task dir. Interactive sessions remain available and run in a karya PTY pane
  bound to the same task worktree.
- **Capability matrix, not lowest common denominator.** An agent without a plan
  mode gets plan emulation (prompt scaffold + "output PLAN.md only"). karya
  surfaces capability gaps instead of hiding them.
- **Mix-and-match steps, swap without loss.** A task spec may pin per-step
  agents (`plan: codex`, `implement: claude`, `review: gemini`), and an agent
  can be replaced mid-task without impacting the work: everything the agent
  needs is on disk (spec, plan, diffs, evidence, transcripts, and the task's
  `MEMORY.md`), agent-agnostic, so the next agent picks up where the last left
  off. Cross-agent review (implementer ≠ reviewer) is the default when ≥2 agents
  are installed.
- **Agents run on karya's tools.** Every agent — headless step or interactive
  pane — runs inside karya's managed environment (§4): karya's isolated `PATH`
  (its mise runtimes, LSP servers, formatters) is active, and the process cwd is
  the task worktree. Agents use the IDE's tools, not whatever happens to be on
  the user's global `PATH`.
- **Prompt assembly** (`internal/prompts`) builds every step prompt from a
  layered context, outermost first, each layer *adding* to (not overriding) the
  next: **global instructions** (user-wide, under the karya prefix) →
  **project instructions** (repo `AGENTS.md` + `.karya/CONTEXT.md`) → **task
  memory** (`.karya/tasks/<id>/MEMORY.md`) → the **spec** + gate feedback. A
  layer overrides an outer one only when explicitly marked
  (`<!-- karya:override -->`); otherwise project enhances global and task
  enhances project. Agents never receive hand-assembled context.

---

## 6. The karya program: one process, one UI, one keymap

karya renders its own screen and owns every keystroke. This section defines the
runtime, the unified keymap, and how Neovim is embedded as the editing engine.

### 6.1 The single-process TUI runtime

karya is a single Go program built on a small, stdlib-only TUI stack. Layers,
bottom-up, each depending only on the layers below and unit-testable in
isolation:

```
internal/term      raw-mode terminal I/O: termios enter/exit, ANSI output,
                   terminfo-lite capability table (keyed on $TERM + truecolor
                   probe), SIGWINCH/size, and an input Decoder ([]byte → Event:
                   Key/Resize/Mouse/Paste; CSI/SS3/UTF-8, lone-Esc timeout)
internal/cellbuf   the screen model: a grid of styled Cells; Set/SetString/Fill/
                   Sub(rect)/Clone; wide-rune width; Diff(prev,next) → minimal
                   cursor-move+write spans. Pure in-memory; the snapshot substrate
internal/tui       the Elm-style runtime: Model{Init/Update/View}, Msg, Cmd; the
                   Program loop (read events → Update → render View to a cellbuf →
                   Diff → flush), frame coalescing (~16 ms), panic-safe restore
internal/pty       PTY host: spawn a child on a pty, resize, reap; sub-package
                   pty/vt is a minimal VT parser ([]byte → screen) so shell/agent
                   panes blit into a cellbuf
internal/nvimrpc   spawn `nvim --embed`, msgpack-RPC over stdio, nvim_ui_attach,
                   reduce redraw events into a Grid model → cellbuf, forward input
                   via nvim_input; includes a minimal stdlib msgpack codec
internal/keymap    the ONE keymap engine: data-driven bindings, modal resolution
                   (Passthrough/Leader/Command/Search), which-key candidates
internal/layout    window/pane/tab tree: splits, focus-by-adjacency, resize, rect
                   computation; PaneContent = editor | terminal | karya view
internal/gitui     the built-in git panel (replaces lazygit) over internal/git
internal/taskview  task board · internal/reviewview + internal/review · gate inbox
internal/diffview  shared unified/side-by-side diff renderer
```

The **update/view split is the load-bearing testing decision**: every UI
component is a `tui.Model` whose `Update(Msg) (Model, Cmd)` is a pure state
transition and whose `View(*cellbuf.Buffer)` is a pure render. All side effects
are `Cmd`s executed off the render path. UI holds no workflow logic — that lives
in the headless packages below it (§7.2).

**Default layout.** Launching `karya` (no file argument) seeds a ready-to-work
three-pane view rather than a bare shell: the **editor** on the left (~65%), a
coding **agent** pane top-right (the first detected agent CLI, running in the
repo), and a **build/test** shell bottom-right. Each pane degrades gracefully —
a missing Neovim falls back to a placeholder, no detected agent falls back to a
shell. The layout is the starting point, not a cage: every pane is splittable,
closable, and zoomable, and karya views (git, tasks, review, gate inbox) open on
demand from the leader. The default is overridable per project/user
(`.karya/` or global config) in a later phase.

### 6.2 The unified keymap (single leader, consistent everywhere)

**Leader: `Ctrl-Space`.** It leads uniformly regardless of what has focus —
editor, shell, or agent CLI — which `Space` alone cannot do when a PTY holds
focus. Neovim keeps its *internal* `Space` leader for editor-local text actions,
but every **IDE-level** action is a `Ctrl-Space` chord intercepted by karya
before Neovim ever sees it.

**Modes** (always shown in the status line): `Passthrough` (keys flow to the
focused pane; only the leader and a tiny always-on set are intercepted) →
`Leader` (entered by `Ctrl-Space`; a which-key popup lists continuations after a
short delay; `Esc` cancels) → `Command`/`Search` for karya views.

The one binding table (`<L>` = `Ctrl-Space`), replacing the former three layers
(tmux prefix, Neovim leader, lazygit):

| Chord | Action | | Chord | Action |
|---|---|---|---|---|
| `<L> h/j/k/l` | Focus pane left/down/up/right | | `<L> 1`–`9` | Go to tab N |
| `<L> H/J/K/L` | Resize focused pane (repeatable) | | `<L> n` / `<L> p` | Next / previous tab |
| `<L> \|` / `<L> -` | Split right / split down | | `<L> c` | New tab |
| `<L> =` | Equalize splits | | `<L> w` | Pane/window switcher |
| `<L> x` | Close focused pane (confirm) | | `<L> t t/n/s` | Task board / new / start |
| `<L> z` | Zoom / unzoom pane | | `<L> g g/c/p` | Git panel / commit / push |
| `<L> e` | Focus editor | | `<L> r` | Review pending gate |
| `<L> b` | Build/test pane; run last | | `<L> a` | Agent inbox / delegate gate |
| `<L> ?` | Full keymap reference | | `<L> Q` | Quit karya (confirm) |
| `Ctrl-Space Ctrl-Space` | Send a literal `Ctrl-Space` to the focused pane | | | |

**Input routing (the crux — karya owns all input).** The `Program` loop hands
every key to `keymap.Engine.Feed(key, Context{Focus, Mode})`:

- full match → dispatch the karya `Action` (handled by `layout`/`gitui`/
  `taskview`/…);
- partial match → `Pending` (arm the which-key popup);
- otherwise → `Forward` to the focused pane: the editor pane forwards to
  `nvimrpc.Client.Input(encodeKey(k))`; a terminal/agent pane writes the raw key
  bytes to its PTY master.

Because the leader is intercepted before forwarding, Neovim only ever receives
unclaimed (text-editing) keys — there is no competing keymap surface. One
reference documents the whole thing: [docs/keymaps.md](docs/keymaps.md).

### 6.3 Neovim embedded as the editing engine

karya spawns `nvim --embed` with `NVIM_APPNAME=karya/nvim` (isolation preserved)
and a **slim, UI-less config**, connects a `nvimrpc.Client` to its stdio, and
calls `nvim_ui_attach(cols, rows, {ext_linegrid=true, rgb=true})` sized to the
editor pane (single global grid to start; `ext_multigrid` deferred until karya
needs to place Neovim windows/floats as independent panes). `redraw`
notifications are reduced into a `Grid` model and blitted into the editor pane's
sub-buffer; the grid cursor is drawn only when the editor pane is focused.
Resizing a pane calls `nvim_ui_try_resize`.

LSP, treesitter, and completion still work — they are Neovim-internal; karya
owns only the UI surface and top-level input, not editing. What remains of
`internal/assets/nvim` is a slim config:

- **Keep:** `core/options.lua`, LSP setup, treesitter, completion, per-language
  servers/tooling, and optional buffer-local `Space` maps for pure text actions.
- **Remove:** everything that is UI or IDE-level keymaps — which-key,
  window/split/leader maps, statusline/tabline/UI plugins, terminal/session/task
  plugins, gitsigns-as-UI, the in-editor tutorial. karya draws all chrome and
  owns all IDE actions.
- **Set at runtime via RPC:** `laststatus=0`, disable Neovim's tabline/
  statusline, `mouse`, `cmdheight` — since karya renders the chrome.

The editor stays Neovim (reuse, don't rewrite). Everything *around* it — the
window manager, git UI, keymaps, and review surfaces — is karya's own, so the IDE
is consistent and testable end to end.

### 6.4 karya for humans: the full editor

The HITL loop is the headline, but a human working in karya must never hit a
wall the moment an agent can't finish — triaging a bug, reading unfamiliar code,
or fixing something by hand needs a real editor, not just a diff viewer. So the
following are first-class, under the one keymap:

- **File navigation.** A fuzzy file finder (`<leader> f`) over the repo's files
  (via ripgrep's file list, or a walk fallback) opens the selection in the editor
  pane. Neovim's own buffer/jumplist navigation works within the editor.
- **Project search.** Live grep (`<leader> /`) over ripgrep — type a query, get
  `file:line` matches, open one at its location in the editor. Both the finder
  and search are karya-native views (like the git panel), so they obey the same
  keymap and open results into the embedded editor.
- **LSP navigation.** The embedded editor exposes the full language surface:
  go-to definition/declaration/type/implementation, references and document/
  workspace symbols, hover and signature help, diagnostics (list + next/prev),
  rename, code action, and format. These are Neovim-native (built-in `vim.lsp`,
  no plugins) under the editor's own `Space` leader; the language servers are the
  ones karya auto-installs (§5).

These are ordinary karya views and editor bindings, tested like everything else
(§8.1); the point of the pillar is that the human path is never a dead end.

---

## 7. Human review UX

Reviewing must be faster than doing the work by hand, or HITL collapses. The
review surface is a native karya view (`internal/reviewview`), rendered in a pane
like everything else:

- **`karya review <task>`** (or `<L> r`) opens the review layout: spec on the
  left, the artifact under review (plan / diff / verification report) on the
  right, feedback buffer below. Keys: `approve` / `reject-with-feedback` / `edit
  artifact` / `delegate to agent`.
- **Plan review:** rendered `PLAN.md` with step list; the human can open the plan
  in the embedded editor (agent treats human edits as binding) or reject with
  feedback.
- **Diff review:** `git diff base...task/<id>` rendered by `internal/diffview`,
  with per-hunk context back to acceptance criteria; `karya review --stat` for a
  quick gate summary (files changed, criteria touched, test delta).
- **Verification review:** the evidence report — which verification commands
  ran, exit codes, failing output excerpts, coverage delta where available —
  not a raw terminal scrollback.
- **Gate inbox:** `karya gate list` (and the status bar segment) shows tasks
  waiting on the human, so multi-task parallelism doesn't bury approvals.
- **Delegation with a paper trail:** any gate can be delegated
  (`karya gate delegate <task> --to gemini`); `STATE.json` records that the
  approval was agent-made, and `karya task audit` shows delegated vs human
  crossings.

---

## 8. Verification & testing strategy

Two layers: how karya tests *tasks*, and how karya tests *itself*.

### Task verification (what agents produce)

1. **Executable acceptance criteria** (spec `Verification` block) run by karya
   in the task worktree; results recorded as `VERIFY-<n>.md` evidence.
2. **Acceptance-test-first option:** when the spec says `tdd: true`, the
   implement step must land failing acceptance tests before implementation, and
   karya verifies the failure signature matches the spec before allowing the
   implement step to continue.
3. **Cross-agent review** as a pre-gate filter (§5) with a reviewer checklist
   derived from the spec.
4. **Regression net:** karya always runs the repo's existing fast suite
   (auto-detected per language or declared in `.karya/project.toml`) at the
   verify gate, even if the spec forgot to.

### karya self-testing (the repo's own discipline)

Hermetic unit tests, `//go:build integration` tests, race detector, `make gate`
before every PR. Specific to the single-process architecture:

- **Worktree/task engine integration tests** drive real `git worktree` in
  `t.TempDir()` repos.
- **Adapter contract tests:** a fake-agent binary (scripted stdin/stdout) runs
  the full `Agent` interface against every adapter's prompt/parsing logic; real
  CLI smoke tests run only in an opt-in `-tags=e2e` suite.

### 8.1 Testing the TUI (the IDE's own UI is tested like a backend)

Every UI component is a `tui.Model` (§6.1). karya's TUI is tested at four levels:

1. **Model unit tests (pure).** Feed `Msg`s (`term.KeyEvent`, custom messages,
   `Cmd` results) to `Update`; assert the resulting model + emitted `Cmd`. This
   is where modal/leader/which-key resolution, focus/resize math, git-panel and
   task-board logic get exhaustive table-driven coverage. No terminal.
2. **Snapshot tests (render).** `View` renders into a `cellbuf.Buffer`; golden
   snapshots pin layouts, status lines, which-key popups, diff rendering, and the
   embedded-Neovim grid blit (from recorded redraw batches). `go test -update`
   regenerates goldens, reviewed in the diff.
3. **PTY integration tests** (`-tags=integration`). The real `karya` binary runs
   on a pseudo-terminal; scripted keystrokes go in, screen contents are scraped
   and asserted. Catches ANSI parsing, resize, and focus behavior.
4. **End-to-end IDE tests** (`-tags=e2e`): full `karya` session in a
   `t.TempDir()` HOME — task created, fake agent plans/implements, gates crossed
   via scripted keystrokes, merge lands. The whole HITL loop, no human, no
   network, no real agent.

**Seams / fakes.** Every impure boundary is an interface with a fake:

- **fake nvim** — a scripted RPC peer that replays canned `redraw` batches for
  given inputs, so `nvimrpc.Grid` rendering and input forwarding are tested with
  no real Neovim. Real Neovim runs behind `-tags=integration`.
- **fake PTY** — an in-memory pipe pair implementing the `pty` interface, so
  terminal-pane models and the `vt` parser test without spawning processes.
- **fake agents** — the existing `agentrun.Runner` fake pattern.
- **fake terminal** — `term.Output` targets a `bytes.Buffer` for byte-exact
  assertions; raw mode is behind an interface (never touches a real fd in tests).

### 8.2 Testing the orchestration backend

Task engine, gates, worktrees, adapters, git service, and marketplaces are
headless Go packages tested headlessly. The TUI is a *client* of these — all
workflow logic lives below the UI, so UI tests stay thin and backend tests carry
the semantic weight. If a behavior can only be tested through the TUI, the
architecture is wrong: push it down.

### 8.3 Editor-engine testing

The slim embedded Neovim config keeps a headless smoke test (drive `nvim
--embed` over RPC, open a buffer, confirm LSP attaches) plus the docs drift test.
Grid-rendering fidelity is covered by `nvimrpc` snapshot tests against recorded
redraw batches, and a real-Neovim integration test that opens a buffer and
compares the rendered screen.

### 8.4 Performance budget (tested, not aspirational)

A terminal IDE must feel instant. Budgets enforced by benchmarks in CI
(`go test -bench`, compared against checked-in baselines; regressions fail):

| Operation | Budget |
|---|---|
| `karya` cold start to first frame | < 300 ms |
| Any CLI command that doesn't touch the network | < 100 ms to first output |
| Keypress → rendered frame | < 16 ms (60 fps) on a 2019 laptop |
| Gate list / task board over 500 tasks | < 100 ms |
| `karya review` open (diff ≤ 5k lines) | < 300 ms |

Rules that protect the budget: no network on any render path (marketplace data
is cached and refreshed in background), no shelling out per keystroke, the
renderer diffs dirty cells only and reuses buffers (no per-cell allocation in the
hot path), and agent/PTY/nvim I/O is always async (the UI never blocks).

---

## 9. Skills marketplace

A **skill** is a portable capability package: a `SKILL.md` (name, description —
which is the trigger — plus procedure) plus optional `scripts/`,
`references/`, `assets/`. This is the format Anthropic/Crush-style agent CLIs
already converged on; karya treats it as the common currency.

- **Registry:** a versioned index (`registry.json` in a git repo; default
  registry hosted at `karya-dev/skills`, user-addable registries via
  `karya skills registry add <url>`). Entries: name, version, description,
  files (content-hashed), license, min agent capabilities.
- **Install:** `karya skills install <name>` downloads, verifies hashes, and
  lands the skill in the karya prefix (`~/.local/share/karya/skills/<name>`) —
  never in the user's global agent dirs.
- **Materialization:** karya symlinks/copies installed skills into each
  detected agent CLI's native skill location (e.g. `~/.claude/skills`,
  crush's skills dir) **only for agents the user has opted in**
  (`.karya/project.toml` or global prefs), and removes them on uninstall.
- **Project skills:** `.karya/skills/` in a repo is auto-visible to all agents
  working on tasks in that repo.
- **Trust:** skills are prompt-bearing code — installs show a diff-able content
  listing, registries can be signature-verified (cosign, later), and `doctor`
  reports installed skills per agent.

## 10. MCP marketplace

Same shape, for MCP servers:

- **Registry:** curated index of MCP servers with install recipes (npm/pipx/
  binary-via-mise), required permissions (fs/network/env), and capability
  metadata.
- **Install:** `karya mcp install <server>` provisions the runtime through the
  isolated mise toolchain and stores the server under the karya prefix.
- **Config generation:** karya renders each agent CLI's **native MCP config
  format** (claude's `.mcp.json`, crush's `crush.json`, gemini's
  `settings.json`, …) from one karya-owned source of truth
  (`mcp.toml` in the karya prefix / `.karya/mcp.toml` per project). One
  install → every agent sees the server. Regenerate on agent add/remove.
- **Permission scoping:** per-project enable/disable; secrets referenced by env
  var name, never written into configs.

---

## 11. Documentation system (for humans *and* agents)

Docs are tooling, not prose. v1.0 formalizes three audiences, three paths:

| Audience | Lives in | Consumed by |
|---|---|---|
| End users | `docs/` (embedded in the binary; synced by `make sync-docs`) | `karya help/docs/tutorial` |
| Contributors (human + agent) | repo root: `DESIGN.md`, `ROADMAP.md`, `PROGRESS.md`, `AGENTS.md` | repo readers, coding agents |
| Per-project agents | `.karya/` in user repos: specs, plans, `CONTEXT.md`, per-task `MEMORY.md` | agents running tasks |
| All agents (user-wide) | karya prefix: `instructions.md` (global) | every agent step, on every project |

Rules:

- **Layered agent instructions (enhance, don't override).** Agent context is
  assembled outermost-first — global → project → task — each layer *adding* to
  the next (§5). The **global** layer (`<karya-config>/instructions.md`, e.g.
  `~/.config/karya/instructions.md`) is your user-wide guidance applied on every
  project; the **project** layer is the repo `AGENTS.md` + `.karya/CONTEXT.md`;
  the **task** layer is `.karya/tasks/<id>/MEMORY.md` + the spec. An inner layer
  overrides an outer one only when explicitly marked (`<!-- karya:override -->`).
  `karya config edit` opens the global file; `karya init` scaffolds the project
  ones.
- **Per-task memory (`MEMORY.md`) is agent-agnostic continuity.** Each task dir
  holds a running `MEMORY.md` — decisions, gotchas, and state accumulated across
  steps — that every prompt includes and any agent may append to. Because it (and
  the plan/diffs/transcripts) live on disk and belong to the *task*, not the
  agent, you can replace the agent working a task at any point without losing
  context (§5).
- **AGENTS.md is generated where useful.** `karya init` in a user repo scaffolds
  `.karya/CONTEXT.md` + repo `AGENTS.md` (build/test/lint commands, conventions)
  by auto-detecting the toolchain — because agent quality is capped by context
  quality. karya dogfoods this on itself.
- **Specs and plans are docs.** They follow the Markdown contract of §3 so both
  audiences parse them; `STATE.json` is the machine-readable shadow.
- **Decision records:** significant design choices land as short ADRs in
  `docs/adr/` (contributor-audience), linked from DESIGN.md. ADR 0001 records
  the single-process / embed-Neovim pivot.
- **Living sync rule (unchanged):** behavior changes update `docs/` + DESIGN.md
  in the same PR; `docs/tutorial.md` must always match reality.
- **Open Knowledge Format (OKF) alignment.** karya's `.karya/` artifacts are
  already OKF-shaped — markdown files with YAML front-matter, organized as a file
  tree, cross-linked by markdown links, produced/consumed by humans *and* agents
  with no SDK (the format is the contract). This mirrors Google's
  [OKF](https://cloud.google.com/blog/products/data-analytics/how-the-open-knowledge-format-can-improve-data-sharing).
  Direction (Phase 6.x): give the `.karya/` knowledge artifacts (`CONTEXT.md`,
  per-task `MEMORY.md`, specs) consistent OKF front-matter (`type`, `title`,
  `description`, `tags`, `timestamp`) and a `karya knowledge export` that emits an
  OKF bundle — so a task's knowledge is portable and shareable across agents,
  tools, and organizations, not locked into karya.

---

## 12. Architecture & package map

```
── Headless workflow engine (reused across the pivot) ──
internal/task/        task lifecycle, state machine, STATE.json, artifacts
internal/spec/        spec parse/validate/render; acceptance-criteria execution
internal/worktree/    git worktree create/lock/teardown per task
internal/gate/        gate model, approvals, delegation, audit log
internal/review/      review sessions: plan/diff/evidence assembly + feedback
internal/git/         headless git service (status/stage/commit/diff/log/push)
internal/agentrun/    Agent interface + claude/codex/crush/gemini/aider/copilot
                      adapters + generic shell adapter; Runner/Git exec seam
internal/prompts/     step prompt assembly from spec + feedback + context
internal/skills/      skill registry client, install, per-agent materialization
internal/mcp/         MCP registry client, install, per-agent config render
internal/config/      XDG paths + karya prefix (isolation); NVIM_APPNAME
internal/tools/        tool detect/install + mise bootstrap
internal/agent/        agent detection (pane-level switching handled in-process)
internal/sandbox/     (later) OS sandbox interface for agent processes

── Single-process TUI runtime (new; replaces tmux + nvim-UI + lazygit) ──
internal/term/        raw-mode terminal I/O, ANSI, terminfo-lite, input decoder
internal/cellbuf/     styled cell grid + minimal diff renderer (snapshot target)
internal/tui/         Elm-style Model/Update/View runtime + Program loop
internal/pty/         PTY host + pty/vt terminal emulator for shell/agent panes
internal/nvimrpc/     nvim --embed msgpack-RPC client + Grid model + msgpack codec
internal/keymap/      unified keymap engine (single leader, modal, which-key)
internal/layout/      window/pane/tab tree: splits, focus, resize, geometry
internal/gitui/       built-in git panel (replaces lazygit)
internal/taskview/    task board view      internal/gateview/  gate inbox view
internal/reviewview/  review layout view   internal/diffview/  diff renderer
```

**Removed by the pivot:** `internal/tmuxx/` (tmux wrapper), `internal/session/`
(tmux layout), `internal/editor/` (send-keys routing + headless plugin sync),
`internal/assets/tmux.conf`, and the UI/keymap subtree of `internal/assets/nvim/`
(the config slims to options + LSP). The lazygit window binding is gone.

Control flow: `karya` launches the `tui.Program`, which builds a `layout` tree
of panes (editor via `nvimrpc`, shells/agents via `pty`, karya views), routes
input through `keymap`, and renders through `cellbuf` + `term`. `karya task
start` → `worktree` + `task` records → `agentrun` drives steps → artifacts land
in the task dir → `gate` blocks transitions → `reviewview` renders artifacts →
human approves → `verify` runs spec verification → merge/ship. Agent panes are
PTYs bound to task worktrees.

## 13. Command surface

```
karya                            Launch the single-process TUI IDE for the cwd
karya edit <file> [line]         Open a file in the embedded editor (also $EDITOR)
karya task new <slug>            Scaffold a task spec; open in editor
karya task refine <id>           Agent-assisted spec sharpening (human approves)
karya task start <id>            Create worktree+branch, run plan step
karya task list|status|show      Task board / detail
karya task abandon|archive <id>  Teardown with/without keeping artifacts
karya plan <id>                  (Re)run the plan step
karya implement <id>             Run/continue the implement step
karya review <id>                Open the review layout for the pending gate
karya gate list|approve|reject|delegate   Gate inbox and crossings
karya verify <id>                Run spec verification, record evidence
karya merge <id>                 Merge task branch (after verify gate) or open PR
karya skills search|install|remove|list|registry   Skills marketplace
karya mcp search|install|remove|list|sync          MCP marketplace
karya init                       Scaffold .karya/ + repo AGENTS.md in a project
karya task audit <id>            Gate history: who/what approved what, when
karya lang|install|update|uninstall|doctor|shellenv|help|docs|tutorial|version
```

## 14. Security & trust model

- Agents never receive credentials from karya; env passthrough is explicit.
- Marketplace content is prompt-bearing code: hash-verified at install,
  content-listed for human review, registries signable (cosign) before v1.0.
- Task worktrees bound the blast radius by construction; OS sandboxing (§4)
  narrows it further once shipped.
- Gate audit trail (`STATE.json` + `karya task audit`) makes delegation
  visible: you can always answer "who approved this line of code?"
- Everything karya installs or generates remains under karya-owned paths;
  `karya uninstall` removes karya, its marketplaces' content, task worktrees,
  and nothing else.

## 15. Migration from the orchestrator design

- **Kept as-is:** system isolation model, embedded assets pipeline, the task
  engine (`task`/`spec`/`worktree`), the agent adapter layer (`agentrun`),
  language tooling, self-update, install/uninstall, doctor (with updated checks).
- **Replaced:** the presentation/orchestration layer. tmux (windowing),
  standalone Neovim UI, and lazygit are gone; karya draws its own screen, embeds
  Neovim as an engine, and ships one keymap. `internal/tmuxx`, `internal/session`,
  and `internal/editor` are removed; `internal/assets/nvim` slims to an engine
  config.
- **New:** the `term`/`cellbuf`/`tui`/`pty`/`nvimrpc`/`keymap`/`layout`/`gitui`
  stack and the karya-native views.
- The transition is phased (ROADMAP.md): docs first, then a walking-skeleton TUI,
  then the editor embed, then panes/git/views — the binary stays launchable at
  every phase and the headless engine never regresses.

## 16. Open questions (resolve in early phases)

1. Cross-machine task resume: are task dirs syncable/committable (`.karya/`
   committed vs gitignored-by-default with opt-in commit)? Default: gitignored
   except `SPEC.md` (committed, it's the contract).
2. `ext_multigrid` vs single-grid: start single-grid; adopt multigrid only when
   karya needs Neovim windows/floats as independent karya panes.
3. Rendering Neovim's cmdline/messages/popupmenu natively (`ext_cmdline` etc.)
   vs letting Neovim draw them on the global grid — defer the `ext_*` split.
4. Registry hosting: central git repo vs OCI artifacts — decide in the skills
   phase based on signing/tooling cost.
5. Sandbox ordering: ship worktree-only isolation first; seatbelt/bubblewrap in
   a hardening phase once the task engine is stable.
