# karya — Roadmap

Phased build order. Each phase is shippable and leaves the binary working.
Track live status in [PROGRESS.md](PROGRESS.md). Full design in
[PLAN.md](PLAN.md).

Legend: ☐ not started · ◐ in progress · ☑ done

---

## Phase 0 — Scaffold & CLI skeleton
**Goal:** a buildable single binary with the command tree stubbed out.

- ☐ Go module, `main.go`, `internal/cli` dispatch (stdlib `flag` for now)
- ☐ `internal/config` — XDG-based karya paths + prefix resolution
- ☐ `internal/version` — version string, `karya version`
- ☐ Stub every command from PLAN §4 (print "not yet implemented")
- ☐ `Makefile` (build/test/fmt/vet), `.gitignore`
- ☐ CI (GitHub Actions: build + vet + test on macOS/Linux)
- ☐ `goreleaser` config for cross-compiled release artifacts

**Done when:** `go build` produces `karya`; `karya version` and `karya --help`
work; every documented command exists as a stub.

---

## Phase 1 — Session orchestration (`dev`) ✅
**Goal:** `karya` launches the isolated tmux IDE session.

- ☑ `internal/tmuxx` — tmux wrapper on dedicated socket `-L karya`
- ☑ Embed + extract `tmux.conf` (`internal/assets`); launch server with `-f`
- ☑ `internal/session.Dev` — editor/agent/build panes + git window + `@ide_*` state
- ☑ Session env: `NVIM_APPNAME=karya/nvim`, `EDITOR/VISUAL/GIT_EDITOR=karya edit`
- ☑ `karya edit <file> [line]` — open a file in the editor pane
- ☑ `karya run [-d dir] <cmd>` / `--focus` — route a command to the build/test pane
- ☑ `-k` recreate / `-q` quit / attach-if-exists
- ☑ Verified: layout, isolated env, `@ide_*` state; default tmux server untouched

**Done when:** `karya` opens the 3-pane layout on a karya-only tmux socket with
the user's tmux/nvim untouched; `karya edit` and `karya run` route correctly.

---

## Phase 2 — Agent management (AI-first core) ✅
**Goal:** first-class agent detection, switching, and memory.

- ☑ `internal/agent` — detect `crush/claude/codex/gemini/aider/copilot`
- ☑ Resolution: flag → per-project pref → single → interactive picker
- ☑ `internal/prefs` — per-project `key=value` store under karya prefix
- ☑ `karya agent status|switch|next|prev|reset|prefs|clear`
- ☑ Wire tmux keybindings (`Ctrl-a A/N/D/P/S`) to karya commands
- ☑ Agent-pane respawn / layout-reset semantics

**Done when:** agent selection, cycling, reset, and per-project memory work
reliably, driven entirely by `karya`.

---

## Phase 3 — Editor integration (embedded Neovim config) ✅
**Goal:** the Neovim IDE ships inside the binary, fully isolated.

- ☑ Vendor the Neovim editor config into `internal/assets/nvim/` for embedding
- ☑ `go:embed` + extract to `~/.config/karya/nvim`; version via `manifest.json`
- ☑ Launch via `NVIM_APPNAME=karya/nvim`; isolated data/state/cache dirs
- ☑ Plugin bootstrap/sync (`editor.SyncPlugins` → `nvim --headless +"Lazy! sync" +qa`)
- ☑ Verify user's `~/.config/nvim` is never read/written (isolation test)

**Done when:** editing works with the full LSP/DAP/git/tasks config, extracted
from the binary, with zero impact on the user's own Neovim.

---

## Phase 4 — Project scaffolding (`new`) ✅
**Goal:** `karya new <lang> <name>` for all six languages.

- ☑ `internal/project` scaffolds: python, java, typescript, go, cpp, rust
- ☑ `git init`; open in an IDE session when inside a karya session
- ☑ `Ctrl-a P` prompt (`lang:name`) → `karya new`

**Done when:** each language produces a complete, buildable scaffold.

---

## Phase 5 — Language & tool management (`lang`, tools) ✅
**Goal:** isolated language/version selection and tool install.

- ☑ `internal/lang` — selector + version discovery from `mise ls-remote`
  (dedup by major/minor, Java distribution ranking), offline fallback
- ☑ Write `languages.local`; generate **isolated** mise config
  (`MISE_GLOBAL_CONFIG_FILE`/`MISE_DATA_DIR` pinned to the karya prefix)
- ☑ `internal/tools` — detect-or-install LSPs/formatters/adapters into the karya
  tool prefix (uv/npm/go/rustup + jdtls/lombok/VSIX downloads); Homebrew-class
  servers are detect-only with a hint (never `brew install`)
- ☑ Always-on servers + per-language selectable servers (PLAN §6.4)
- ☑ `karya lang [list|add|remove|all]` + interactive selector
- ☑ **Self-contained core runtime** (2026-07-31) — detect-or-install now extends
  to tmux, Neovim, and the toolchain managers via a karya-vendored, isolated
  mise (`internal/tools/{mise,bootstrap}.go`; `EnsureMise`/`EnsureCore`/
  `EnsureToolchains`). A fresh machine with only the binary bootstraps the whole
  stack; `ActivateManagedEnv` lets karya run its own shim-backed tools. (PLAN §6.4)

**Done when:** selecting a language installs its tooling into the karya prefix
without modifying Homebrew or the user's global mise.

---

## Phase 6 — Install / update / uninstall & self-update ✅
**Goal:** lifecycle management, including binary self-replacement.

- ☑ `karya install` — extract configs, fetch tools, no user-setting changes
- ☑ `karya update [--check]` — self-replace binary + re-extract configs + tools + `Lazy! sync`
- ☑ `karya uninstall` — remove karya prefix + binary only (confirmed, `-y` to skip)
- ☑ GitHub Releases integration + checksum verification + atomic replace
  (`internal/update`, hermetically tested via httptest)
- ☑ `curl | sh` install script (`scripts/install.sh`)
- ☑ `karya shellenv` (opt-in PATH/alias/EDITOR; session toolchain stays session-scoped)

**Done when:** a user can install via one command, update in place, and fully
uninstall leaving no trace beyond their own pre-existing config.

---

## Phase 7 — Embedded help, self-guided tutorial, doctor & distribution
**Goal:** production-ready release where the docs *ship inside the binary* — a
user who has only `karya` on their PATH can learn and use it fully offline,
without the repo or a browser.

### Embedded help & tutorial (in the binary, not just `docs/`)
- ☑ `internal/assets` — `go:embed` the user docs (`docs/tutorial.md`,
  `docs/keymaps.md`) so they travel with the binary (`make sync-docs` vendors them)
- ☑ `karya help [command]` — rich, per-command help beyond the flag usage `flag`
  prints, pointing at the embedded docs; `karya help topics` lists the commands
- ☑ `karya tutorial` — a **self-guided, self-working** tutorial launched from the
  binary: numbered lessons a user can step through (`karya tutorial [lesson]`),
  each runnable against a throwaway sandbox so commands actually execute and are
  verified (isolation, scaffold, git-init, embedded docs, tmux, agents)
- ☑ `karya docs [topic]` — browse the embedded docs offline (pager/`$PAGER`)
- ☑ Single source of truth: `docs/*.md` are the source, embedded at build time; a
  test asserts the embedded content stays in sync with `docs/`
- ☑ `Ctrl-a ?` keybinding opens the in-session help (key map & command reference)

### Doctor & distribution
- ☑ `karya doctor` — tools, versions, isolation checks, per-language tooling
- ☐ `docs/` complete (isolation, commands, languages, troubleshooting)
- ☑ Shell completions (`karya completion <bash|zsh|fish>`)
- ☑ Homebrew tap — cross-platform formula (macOS + Linux, amd64 + arm64) generated
  by `scripts/update-formula.sh` from the release checksums. Verified with `v0.1.0`
  (`HomebrewFormula/karya.rb` on `main`, checksums matching the release). On each
  tag the release workflow opens a formula PR and auto-merges it (via the
  `RELEASE_TOKEN` PAT), keeping the tap current hands-off.
- ☑ Release automation — GoReleaser (`.goreleaser.yaml`) + `.github/workflows/release.yml`
  on `v*` tags: cross-compiled tarballs + checksums, `install.sh` attached, changelog
  from commits. Proven end-to-end by the `v0.1.0` release. **Remaining:** tag `v0.2.0`.

### Provenance cleanup (final pass — karya stands alone) ✅
- ☑ Describe karya on its own terms across the **entire repo** — karya is the
  product, not a port. Covered both shipped surfaces (`--help`, README, `docs/`,
  in-code comments) **and** the internal design log (`PLAN.md`, `PROGRESS.md`,
  `AGENT.md`): consolidation-map and "port of X" language now describes karya's
  own commands and subsystems. Historical origins live in git history, not the tree.
- ☑ Severed the build-time dependency on sibling repositories: the vendored
  `internal/assets/nvim/` is now the sole source of truth (edited directly), the
  embedded editor config drives karya's own `karya run`/`karya install` instead of
  external scripts, and the old vendoring script is retired — a clean checkout
  builds with no sibling repos.

**Done when:** clean install → working AI IDE → `karya doctor` all-green on a
fresh macOS machine, and `karya help`/`karya tutorial` teach the whole workflow
**offline from the binary alone**, with no repo checkout or network needed —
and karya's tree stands alone as its own product.

---

## Phase 8 — Cohesion & UX (make karya feel like one product) ✅
**Goal:** the amalgamation of Neovim + tmux + agents reads as a single entity —
consistent, non-bloated keymaps; the agent fused into the editor; the
scaffold → implement → ship loop closed with the agent's help. Ships in `v0.2.0`.

- ☑ **Unified keymaps** — one context-aware `<leader>c` "Code" group, identical in
  every language (no per-language `<leader>p/o/j/r/C/y` prefixes; `<leader>ct`
  always runs the nearest test, etc.). `util/langmaps.lua` is the single
  registration point; the overlapping `<leader>T`/`<leader>m`/`<leader>W` groups are
  folded in; close-buffer moved to `<leader>x`. ~8 groups collapse to
  `<leader>c` + `<leader>a`.
- ☑ **Editor↔agent bridge** — `<leader>a` group + `karya agent send`/`focus`: push
  the buffer, a visual selection, a diagnostic, or a file reference into the agent
  pane (pasted unsubmitted). The agent feels native, not a bolted-on CLI.
- ☑ **Agent-driven ship** — `karya ship [--push --pr --no-verify]` (`<leader>gc`,
  `Ctrl-a G`): stage → active agent authors a Conventional-Commit message
  (headless where available, conversational fallback otherwise) → confirm →
  commit → push / PR. Git plumbing stays deterministic in `internal/ship`.
- ☑ **Cohesion polish + guardrail** — panes read `karya · editor/agent/build`;
  `karya keys` shows the reference; an `//go:build integration` test drives headless
  Neovim to assert the identical `<leader>c` interface across all languages and that
  close-buffer moved off `<leader>c`, so consistency can't silently regress.

**Done when:** the same keys drive every language; the agent takes editor context
and authors commits; and the guardrail + full gate are green. **Ready for `v0.2.0`.**

---

## Human-in-the-loop, AI-agents-first arc (Phases 9–13)

karya's next arc turns it from "an agent in a pane" into a **human-in-the-loop,
AI-agents-first IDE**: agents are the primary way work gets done and the human
directs and reviews them. It adds a *second* kind of isolation on top of karya's
existing **environment** isolation — **task-level** isolation: each unit of agent
work in its own git worktree/branch (`karya/<task-id>`) under karya-owned dirs, so
changes are contained and reviewable before they touch the user's real branch.
Full design in [PLAN.md](PLAN.md) §6.6. Locked decisions: pluggable agent engine
(BYO-CLI first, native drops in behind one interface); gates before fleet; all
four HITL gates (plan approval, diff review, checkpoint/rollback, permission
prompts).

## Phase 9 — Agent-runner interface (make the engine pluggable) ✅
**Goal:** one interface for every agent engine so nothing downstream depends on a
specific CLI. Foundational refactor; no user-visible behavior change.

- ☑ `agent.Runner` interface — `Name`, `InteractiveCommand` (pane launch),
  `Headless(ctx, dir, prompt)` (one-shot); `ErrNoHeadless` sentinel + a
  `SupportsHeadless` capability probe (`internal/agent/runner.go`, `headless.go`)
- ☑ `cliRunner` wraps each BYO-CLI behind the interface; `headlessArgv` is now the
  internal lookup table it consumes
- ☑ Real consumers wired: `Manager.launch` starts the agent via
  `InteractiveCommand`; `cli/ship.go` authors commit messages via
  `Runner.Headless` — identical behavior, engine-agnostic path
- ☑ Native engine seam reserved for Phase 13 (interface only)

**Done when:** both interactive and headless paths run through `agent.Runner`,
the full gate is green, and existing agent/ship behavior is unchanged.

---

## Phase 10 — Task model + isolated task workspace (the spine) ✅
**Goal:** the **task** becomes karya's primary noun; each gets an isolated
worktree/branch.

- ☑ `internal/task` — `Task` (id, title, prompt, agent, status, branch, worktree,
  repo, timestamps) + lifecycle statuses; per-project JSON `Store`
  (List/Get/Save-upsert/Delete) under `config.Paths.TasksDir()`
- ☑ `internal/worktree` — git worktree management behind a consumer-defined
  `Runner` (satisfied by `ship.ExecRunner`): `Add` creates branch `karya/<id>`
  checked out under `config.Paths.WorktreesDir()`; `Remove` force-removes the
  worktree + branch + residual dir; `ProjectSlug` groups tasks per repo
- ☑ `karya task new "<prompt>" [--agent]`, `task list`/`tasks`, `task switch <id>`,
  `task rm <id> [-y]`; order-independent flag parsing (prompt is free-form); the
  agent works **inside the worktree** session, not the raw cwd. (`--plan` lands
  with the plan gate in Phase 11.)
- ☑ Isolation proven: real-git integration test asserts the checkout lives under
  the karya root (never in the user tree), the branch is namespaced, and `Remove`
  leaves nothing behind; end-to-end smoke confirms the user repo stays pristine

---

## Phase 11 — Human-in-the-loop gates (all four) ✅
**Goal:** the human directs and reviews; nothing lands unreviewed.

- ☑ **Plan approval** — `task new --plan` drafts a plan via the agent's headless
  mode and parks the task at `awaiting-plan`; `task plan` shows it;
  `task approve-plan` gates `awaiting-plan → working`. (Agents with no headless
  mode still park at the gate for a hand-written/approved plan.)
- ☑ **Diff review before apply** — `task review` stages the worktree and shows the
  whole task diff against its recorded base commit (user branch still untouched);
  `task merge` commits + merges `karya/<id>` into the project branch (`--no-ff`),
  `task reject` marks it rejected. Reuses/extends `ship.Git`
  (`RevParse`/`CommitAll`/`DiffCachedAgainst`/`Merge`/`ResetHard`).
- ☑ **Checkpoint & rollback** — `task checkpoint [label]` commits a restorable
  snapshot on the branch; `task rewind [index|sha]` resets the worktree to one.
  (Explicit per the honest note below — automatic per-turn checkpoints need the
  native engine to observe turns, Phase 13.)
- ☑ **Permission prompts** — `gateAction` confirms karya-initiated merge/push/
  rewind, with a per-project allowlist (`task allow <action>`) and `-y` to skip.
  *Caveat, stated in-code:* this gates only karya's **own** actions; per-tool-call
  gating of a BYO-CLI's internal calls needs the native engine (Phase 13).
- ☑ Gate commands default to the **current task** inside a `task-<id>` session
  (no id needed). `<leader>k` "Karya Tasks" nvim group (Terminal already owns
  `<leader>t`) drives new/list/review/merge/reject/checkpoint/rewind in the
  build pane; the Phase-8-style headless-nvim guardrail asserts it stays bound.

---

## Phase 12 — Fleet (parallel, worktree-isolated agents) ✅
**Goal:** many agents at once, once one task reviews cleanly.

- ☑ Concurrent tasks, each its own worktree/branch/agent/session — this falls out
  of the Phase 10 model: every `karya task new` is an independent worktree +
  `task-<id>` session, so a fleet just works (verified: 3 tasks, 3 isolated
  worktrees + `karya/<id>` branches side by side).
- ☑ `karya task dashboard` — the fleet view: a numbered table of every task with
  its live status; pick a number/id to switch to it. Bound to **`Ctrl-a T`** via a
  tmux popup so the switch lands in the underlying client.
- ☑ `task switch` attaches a task's session (Phase 10); the diff-review/merge gates
  (Phase 11) are the per-task review queue.

---

## Phase 13 — Native agent engine (second Runner impl)
**Goal:** the Phase 9 pluggability pays off.

- ☐ `nativeRunner` behind `agent.Runner` using the Claude API (tool-use: edit,
  run, read); config for keys/models; **BYO-CLI stays the default**
- ☐ Unlocks true per-tool-call **permission prompts** + streaming plan/diff,
  closing the Phase 11 caveat

**Status:** the interface (Phase 9) already exists; this is an additive
implementation, no consumer churn.
