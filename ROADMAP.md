# karya — Roadmap to v1.0

Phased build order for the human-in-the-loop agent IDE. Each phase is shippable
and leaves the binary working. Full design: [DESIGN.md](DESIGN.md). Live
status: [PROGRESS.md](PROGRESS.md). The v0 roadmap (Phases 0–8, shipped) is in
[archive/v0/ROADMAP.md](archive/v0/ROADMAP.md).

Legend: ☐ not started · ◐ in progress · ☑ done

---

## Phase A — Task engine foundation
**Goal:** the task is a real, persistent, isolated unit of work.

- ☑ `internal/task` — task lifecycle, `STATE.json`, artifact store
  (`.karya/tasks/<id>/`); reworks the 9–14-era prefix-based store
- ☑ `internal/spec` — spec parse/validate/render (front-matter + Objective /
  Acceptance criteria / Context / Constraints / Verification)
- ☑ `internal/worktree` — git worktree create/lock/teardown per task, branch
  `task/<id>`, base-ref selection, dirty-tree safety; reworks the 9–14-era
  `karya/<id>` manager
- ☑ `karya task new|list|status|show|start|abandon` (CLI only; TUI in Phase C)
- ☑ Integration tests: real `git worktree` in `t.TempDir()` repos
- ☑ `karya init` — scaffold `.karya/` + repo `AGENTS.md` by toolchain detection

**Done when:** a task can be created from a spec, gets an isolated worktree,
and its state survives restarts — with zero effect on the user's working tree.

---

## Phase B — Agent adapter layer
**Goal:** one interface drives every coding-agent CLI, headless-first.

- ☐ `internal/agentrun` — `Agent` interface, `Caps` matrix, transcripts to task dir
- ☐ Adapters: claude, codex, crush, gemini, aider, copilot + generic shell adapter
- ☐ Plan-mode mapping: native where available, prompt-scaffold emulation otherwise
- ☐ `internal/prompts` — step prompt assembly (spec + feedback + repo context)
- ☐ Adapter contract tests via scripted fake-agent binary; opt-in `-tags=e2e`
  real-CLI smoke suite
- ☐ Fold v0 `internal/ship` into `agentrun`; pane switching stays in `internal/agent`

**Done when:** `karya plan <id>` and `karya implement <id>` run any detected
agent headlessly in the task worktree, capturing PLAN.md and transcripts.

---

## Phase C — Gates, review UX, and the karya TUI
**Goal:** mandatory human gates with a fast, modal, keyboard-first review flow.

- ☐ `internal/gate` — gate model, approve/reject-with-feedback, delegation,
  audit trail; state machine enforcement (DESIGN.md §2)
- ☐ `internal/review` — plan / diff / verification-evidence rendering
- ☐ `internal/tui` — TUI core (cell buffer, input parser, modal keymap engine,
  which-key popups) with pure update/view split
- ☐ Views: task board, review layout, gate inbox; Neovim-matching keymaps
- ☐ `karya review <id>`, `karya gate list|approve|reject|delegate`
- ☐ TUI tests: model unit tests, golden snapshots, PTY integration tests
  (DESIGN.md §7.1)
- ☐ tmux sidebar: live task/gate status inside `karya dev` sessions

**Done when:** the full loop spec → plan → gate → implement → gate runs with
human approvals in the TUI, and every crossing is recorded.

---

## Phase D — Verification & merge
**Goal:** karya certifies; agents never self-certify.

- ☐ Executable `Verification` blocks: karya runs them in the task worktree,
  records `VERIFY-<n>.md` evidence (exit codes + excerpts)
- ☐ `tdd: true` acceptance-test-first flow with failure-signature check
- ☐ Cross-agent reviewer step (implementer ≠ reviewer) as pre-gate filter
- ☐ Regression net: auto-detected per-language fast suite at the verify gate
- ☐ `karya verify <id>` + `karya merge <id>` (merge or PR mode, post-gate only)
- ☐ Performance benchmarks in CI with checked-in baselines (DESIGN.md §7.4)

**Done when:** a task reaches DONE only through verify-gate evidence, and merge
lands on the user's terms (direct merge or PR).

---

## Phase E — Skills marketplace
**Goal:** portable SKILL.md packages, installed once, visible to every agent.

- ☐ `internal/skills` — registry client, hash-verified install into karya prefix
- ☐ Default registry (git-backed index) + `karya skills registry add <url>`
- ☐ Per-agent materialization (opt-in per agent CLI) + removal on uninstall
- ☐ Project-local `.karya/skills/` auto-visible to task agents
- ☐ `karya skills search|install|remove|list` + TUI browser
- ☐ `karya doctor` reports installed skills per agent

**Done when:** a skill installs from the registry and is usable by every opted-in
agent in task runs, fully inside karya-owned paths.

---

## Phase F — MCP marketplace
**Goal:** one MCP install → every agent's native config.

- ☐ `internal/mcp` — registry client, runtime provisioning via isolated mise
- ☐ `mcp.toml` source of truth (global + per-project `.karya/mcp.toml`)
- ☐ Native config renderers: claude `.mcp.json`, crush `crush.json`, gemini
  `settings.json`, …; regenerate on agent add/remove
- ☐ Permission scoping per project; secrets by env-var reference only
- ☐ `karya mcp search|install|remove|list|sync` + TUI browser

**Done when:** installing an MCP server makes it available to all detected
agents without the user editing any agent config by hand.

---

## Phase G — Hardening, sandbox, v1.0
**Goal:** production-trustworthy HITL IDE.

- ☐ `internal/sandbox` — seatbelt (macOS) / bubblewrap (Linux) confinement of
  agent processes to the task worktree
- ☐ Registry signing verification (cosign) for skills + MCP
- ☐ `karya task audit` — full gate/delegation history report
- ☐ Dogfood: karya developed through karya tasks for one full cycle
- ☐ Docs completion: tutorial chapter on the task workflow, keymaps, ADRs
  (`docs/adr/`) for Phase A–F decisions
- ☐ Tag v1.0

**Done when:** sandboxed task execution, signed marketplace content, audit
trail, and docs that match behavior. Ship v1.0.
