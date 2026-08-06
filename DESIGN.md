# karya — v1.0 Design Plan: The Human-in-the-Loop Agent IDE

> **karya** (कार्य — "work/task") is a **human-in-the-loop, agent-based IDE**
> delivered as a single self-contained Go binary. Humans set intent and review
> at explicit gates; coding agents plan, implement, and verify inside isolated
> task environments. karya orchestrates Neovim, tmux, and every major coding
> agent CLI behind one consistent workflow — **without touching any of the
> user's existing settings**.

This document supersedes the v0 design (`archive/v0/PLAN.md`). v0 shipped the
foundation: system-level isolation, embedded Neovim/tmux configs, agent
detection/switching, language tooling, and self-update. v1.0 builds the
**workflow layer** on top of that foundation — it is an evolution of the current
codebase, not a rewrite. Track execution in [ROADMAP.md](ROADMAP.md) and
[PROGRESS.md](PROGRESS.md); engineering rules in [AGENTS.md](AGENTS.md).

---

## 1. Product vision

Coding agents today are chat panes bolted onto editors. The human copy-pastes
intent in, eyeballs a wall of diff, and hopes the tests that ran were the right
ones. The loop is implicit, the artifacts (plan, diff, test evidence) are
ephemeral, and every agent CLI speaks a different dialect.

karya v1.0 makes the loop explicit and first-class:

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

### Design pillars

1. **The Task is the unit of work.** Not a chat session, not a branch — a task:
   a spec, an isolated worktree, an agent, artifacts, and a gate history.
2. **Human gates are mandatory, not advisory.** Plan, implementation, and
   verification each require an explicit human approval to advance. The human
   can delegate a gate to an agent, but the delegation is recorded.
3. **Agent-agnostic.** claude, codex, crush, gemini, aider, copilot, and future
   agents are interchangeable behind one adapter interface — including mixing
   agents across steps (plan with one, implement with another).
4. **Isolation at two levels.** System-level (v0: never touch user dotfiles,
   Homebrew, global mise) and task-level (git worktree + branch per task;
   agent changes are physically incapable of landing on your working tree
   until you merge them).
5. **Artifacts over chat logs.** Specs, plans, diffs, and verification reports
   are Markdown/JSON files in the repo, diffable and greppable by humans *and*
   re-ingestible by agents.
6. **Terminal-native and fast.** Everything is a TUI: Neovim is the editor
   engine, tmux is the window manager, and karya's own surfaces (task board,
   review layouts, gate inbox) are karya-native TUI views in tmux panes. No
   Electron, no browser, no GUI. Startup and gate operations feel instant
   (§7.4 performance budget).
7. **Modal and keyboard-first, inspired by Neovim.** karya's TUI views borrow
   Neovim's grammar: modes (normal/leader/feedback-insert), a leader key,
   which-key-style discoverability, and the *same* keymap scheme as the
   embedded editor config — muscle memory transfers between editor and karya
   views (§6.1).
8. **Reuse, don't rewrite.** Neovim, tmux, git worktrees, and agent CLIs do the
   heavy lifting; karya orchestrates. Single static binary, no CGO.

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
- `karya task list` shows every task and its state; the tmux sidebar shows it
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

System-level isolation (v0, unchanged): everything under karya-owned XDG dirs,
`NVIM_APPNAME=karya/nvim`, dedicated tmux socket, opt-in shellenv, vendored
mise. See `archive/v0/PLAN.md` §2 — it still governs.

Task-level isolation (new in v1.0):

- **One git worktree per task.** `karya task start` creates
  `git worktree add <karya-prefix>/worktrees/<project>/<id> -b task/<id>`.
  The agent's tmux pane(s) and any headless runs are locked to that worktree
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
  task dir. Interactive pane sessions remain available (v0 behavior) and are
  attached to the same task.
- **Capability matrix, not lowest common denominator.** An agent without a plan
  mode gets plan emulation (prompt scaffold + "output PLAN.md only"). karya
  surfaces capability gaps instead of hiding them.
- **Mix-and-match steps.** Task spec may pin per-step agents
  (`plan: codex`, `implement: claude`, `review: gemini`). Cross-agent review
  (implementer ≠ reviewer) is the default when ≥2 agents are installed.
- **Prompt assembly** (`internal/prompts`) builds step prompts from the spec +
  feedback + repo docs; agents never receive hand-assembled context.

---

## 6. Human review UX

Reviewing must be faster than doing the work by hand, or HITL collapses. The
review surface lives in the tmux IDE (Neovim) and the CLI:

- **`karya review <task>`** opens the review layout: spec on the left, the
  artifact under review (plan / diff / verification report) on the right,
  feedback buffer below. Keys: `approve` / `reject-with-feedback` / `edit
  artifact` / `delegate to agent`.
- **Plan review:** rendered `PLAN.md` with step list; human can edit the plan
  directly (agent treats human edits as binding) or reject with feedback.
- **Diff review:** `git diff base...task/<id>` in delta with per-hunk context
  back to acceptance criteria; `karya review --stat` for a quick gate summary
  (files changed, criteria touched, test delta).
- **Verification review:** the evidence report — which verification commands
  ran, exit codes, failing output excerpts, coverage delta where available —
  not a raw terminal scrollback.
- **Gate inbox:** `karya gate list` (and the tmux status segment) shows tasks
  waiting on the human, so multi-task parallelism doesn't bury approvals.
- **Delegation with a paper trail:** any gate can be delegated
  (`karya gate delegate <task> --to gemini`); `STATE.json` records that the
  approval was agent-made, and `karya task audit` shows delegated vs human
  crossings.

### 6.1 The karya TUI: modal, keyboard-first, Neovim-inspired

karya's own surfaces — the task board, review layout, gate inbox, marketplace
browsers — are **native TUI programs** (`internal/tui/`), rendered in tmux
panes and fully operable from the keyboard. They adopt the interaction grammar
of the embedded Neovim config so the whole IDE feels like one program:

- **Modes, not menus.** Views have a normal mode (navigation/actions), a leader
  layer (`<Space>` …, matching the editor's leader), and an insert/feedback
  mode for typing rejection feedback or spec edits. `Esc` always returns to
  normal. No modal trap states; the mode is always visible in the status line.
- **The same keymap scheme as the editor.** Pane/window movement uses the same
  keys as the tmux+nvim config; which-key-style popup discovery (press leader,
  wait, see options) is built into the TUI, mirroring the editor's which-key.
  One keymap reference (`docs/keymaps.md`) documents editor, tmux, and TUI.
- **Vim motions where they fit.** `j/k/g/G//`/`n`/`q` in lists and readers;
  `dd`-style verbs in the task board (e.g. abandon) with confirm.
- **Mouse optional, never required.**
- **Technique:** a small, stdlib-flavored TUI core over raw ANSI/terminfo,
  kept dependency-free per the locked zero-external-deps rule (a thin internal
  cell-buffer + input-parser; the v0 codebase already shells out to tmux for
  all windowing, so the TUI core only handles its own pane). If this proves
  limiting, adopting a single well-vetted TUI library is a locked-decision
  change requiring an ADR — it is not made casually.

The editor itself stays Neovim (reuse, don't rewrite). The TUI covers only
what Neovim/tmux don't already do well: task state, gates, diffs-as-artifacts,
marketplaces.

---

## 7. Verification & testing strategy

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

Unchanged from v0 engineering rules (see AGENTS.md): hermetic unit tests,
`//go:build integration` tests on throwaway tmux sockets/nvim instances, race
detector, `make gate` before every PR. New in v1.0:

- **Worktree/task engine integration tests** drive real `git worktree` in
  `t.TempDir()` repos.
- **Adapter contract tests:** a fake-agent binary (scripted stdin/stdout) runs
  the full `Agent` interface against every adapter's prompt/parsing logic; real
  CLI smoke tests run only in an opt-in `-tags=e2e` suite (needs the actual
  agent installed, skipped by default, run in CI on a schedule).

### 7.1 Testing the TUI (the IDE's own UI is tested like a backend)

A UI that isn't tested rots. karya's TUI is tested at four levels:

1. **Model unit tests (pure).** The TUI follows an update/view split (model in,
  render out — Elm-style, no framework). State transitions are pure functions:
  feed key events as data, assert model + emitted commands. No terminal needed.
  This is where modal/leader/which-key behavior gets exhaustive table-driven
  coverage.
2. **Snapshot tests (render).** Views render to an in-memory cell buffer;
  golden-file snapshots pin layout, status lines, and which-key popups. Golden
  updates are an explicit `go test -update` action reviewed in the diff.
3. **PTY integration tests.** The real TUI binary runs on a pseudo-terminal;
  scripted keystrokes go in, screen contents are scraped and asserted
  (teatest-style, hand-rolled to stay dependency-free). Catches what pure model
  tests can't: ANSI parsing, resize, focus loss, tmux pane interactions.
4. **End-to-end IDE tests** (`-tags=e2e`): full `karya` session in a
  `t.TempDir()` HOME — task created, fake agent plans/implements, gates
  crossed via scripted keystrokes, merge lands. The whole HITL loop, no human,
  no network, no real agent.

### 7.2 Testing the orchestration backend

Task engine, gates, worktrees, adapters, marketplaces are headless Go packages
and are tested headlessly (§7 intro). The TUI is a *client* of these packages —
all workflow logic lives below the UI, so UI tests stay thin and backend tests
carry the semantic weight. If a behavior can only be tested through the TUI,
the architecture is wrong: push it down.

### 7.3 Editor/testing infrastructure (v0, kept)

The embedded Neovim config keeps its existing guarantees: the headless keymap
guardrail (`internal/assets/keymaps_integration_test.go`), tutorial guard, and
docs drift test. New TUI views that embed nvim (e.g. diff review) reuse the
headless-nvim harness.

### 7.4 Performance budget (tested, not aspirational)

A terminal IDE must feel instant. Budgets enforced by benchmarks in CI
(`go test -bench`, compared against checked-in baselines; regressions fail):

| Operation | Budget |
|---|---|
| `karya` cold start to attached session | < 500 ms (warm tmux server) |
| Any CLI command that doesn't touch the network | < 100 ms to first output |
| TUI keypress → rendered frame | < 16 ms (60 fps) on a 2019 laptop |
| Gate list / task board over 500 tasks | < 100 ms |
| `karya review` open (diff ≤ 5k lines) | < 300 ms |

Rules that protect the budget: no network on any render path (marketplace data
is cached and refreshed in background), no shelling out per keystroke, TUI
renders dirty cells only, and agent I/O is always async (the UI never blocks on
an agent).

---

## 8. Skills marketplace

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

## 9. MCP marketplace

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

## 10. Documentation system (for humans *and* agents)

Docs are tooling, not prose. v1.0 formalizes three audiences, three paths:

| Audience | Lives in | Consumed by |
|---|---|---|
| End users | `docs/` (embedded in the binary; synced by `make sync-docs`) | `karya help/docs/tutorial` |
| Contributors (human + agent) | repo root: `DESIGN.md`, `ROADMAP.md`, `PROGRESS.md`, `AGENTS.md` | repo readers, coding agents |
| Per-project agents | `.karya/` in user repos: specs, plans, `CONTEXT.md` | agents running tasks |

Rules:

- **AGENTS.md is generated where useful.** `karya init` in a user repo scaffolds
  `.karya/CONTEXT.md` + repo `AGENTS.md` (build/test/lint commands, conventions)
  by auto-detecting the toolchain — because agent quality is capped by context
  quality. karya dogfoods this on itself.
- **Specs and plans are docs.** They follow the Markdown contract of §3 so both
  audiences parse them; `STATE.json` is the machine-readable shadow.
- **Decision records:** significant design choices land as short ADRs in
  `docs/adr/` (contributor-audience), linked from DESIGN.md — replacing the v0
  pattern of burying decisions in PROGRESS.md.
- **Living sync rule (unchanged):** behavior changes update `docs/` + DESIGN.md
  in the same PR; `docs/tutorial.md` must always match reality.

---

## 11. Architecture & package map

New/changed packages (v0 packages unchanged unless noted):

```
internal/task/        task lifecycle, state machine, STATE.json, artifacts
internal/spec/        spec parse/validate/render; acceptance-criteria execution
internal/worktree/    git worktree create/lock/teardown per task
internal/gate/        gate model, approvals, delegation, audit log
internal/review/      review sessions: plan/diff/evidence rendering + feedback
internal/agentrun/    Agent interface + claude/codex/crush/gemini/aider/copilot
                      adapters + generic shell adapter (supersedes v0's
                      detect-only internal/agent for task work; pane-level
                      switching stays in internal/agent)
internal/prompts/     step prompt assembly from spec + feedback + context
internal/skills/      skill registry client, install, per-agent materialization
internal/mcp/         MCP registry client, install, per-agent config render
internal/tui/         karya-native TUI core (cell buffer, input parser, modal
                      keymap engine) + views: task board, review layout, gate
                      inbox, marketplace browsers. Pure update/view split so
                      models are unit-testable without a terminal (§7.1)
internal/sandbox/     (later) OS sandbox interface for agent processes
internal/agent/       v0 pane management (kept); gains task awareness
internal/ship/        v0 headless runner (folded into agentrun as adapters land)
```

Control flow: `karya task start` → `worktree` + `task` records → `agentrun`
(drive steps) → artifacts land in the task dir → `gate` blocks transitions →
`review` renders artifacts → human approves → `verify` runs spec verification →
merge/ship. The tmux session (v0 `session`) becomes the host UI: a task sidebar
window, review layouts, and agent panes bound to task worktrees.

## 12. Command surface (v1.0 additions)

```
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
```

All v0 commands (`dev`, `agent`, `edit`, `run`, `new`, `lang`, `install`,
`update`, `uninstall`, `doctor`, `shellenv`, …) are unchanged in behavior;
`agent` and `dev` gain task awareness.

---

## 13. Security & trust model

- Agents never receive credentials from karya; env passthrough is explicit.
- Marketplace content is prompt-bearing code: hash-verified at install,
  content-listed for human review, registries signable (cosign) before v1.0.
- Task worktrees bound the blast radius by construction; OS sandboxing (§4)
  narrows it further once shipped.
- Gate audit trail (`STATE.json` + `karya task audit`) makes delegation
  visible: you can always answer "who approved this line of code?"
- Everything karya installs or generates remains under karya-owned paths;
  `karya uninstall` removes karya, its marketplaces' content, task worktrees,
  and nothing else (v0 guarantee, extended).

## 14. Migration from v0

- **Kept as-is:** isolation model, embedded assets pipeline, tmux orchestration,
  editor config, language tooling, self-update, install/uninstall, doctor.
- **Extended:** `internal/agent` (pane mgmt) gains task binding; `internal/ship`
  folds into `agentrun`; prefs store gains gate/delegation preferences.
- **Replaced:** v0's "chat pane is the workflow" model. Panes remain, but the
  task engine is the workflow.
- **No breaking CLI changes** before v1.0; new commands are additive.

## 15. Open questions (resolve in early phases)

1. Cross-machine task resume: are task dirs syncable/committable (`.karya/`
   committed vs gitignored-by-default with opt-in commit)? Default: gitignored
   except `SPEC.md` (committed, it's the contract).
2. Multi-agent parallel implementation on one task (competing diffs) — v1.1+.
3. Registry hosting: central git repo vs OCI artifacts — decide in Phase E
   based on signing/tooling cost.
4. Sandbox ordering: ship worktree-only isolation first; seatbelt/bubblewrap in
   a hardening phase once the task engine is stable.
