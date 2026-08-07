# AGENTS.md — Engineering guide for karya

This is the authoritative guide for anyone — human or AI — working on karya. Read
it fully before writing code, then read [PROGRESS.md](PROGRESS.md) for the exact
resume point. It defines **how** we build karya so the project stays correct,
maintainable, well-tested, well-documented, and welcoming to contributors.

## What karya is
A **human-in-the-loop, agent-based IDE** shipped as a **single Go binary** — one
process that owns the terminal. karya draws its own screen (window/pane/tab
manager, git panel, task/gate/review views, PTY-hosted shells and agent CLIs) and
**embeds Neovim as the text-editing engine** over msgpack-RPC (`nvim --embed`),
rendering Neovim's grid into karya's own cell buffer. Everything is driven by one
unified keymap under a single leader. It installs/updates/uninstalls itself
**without touching any of the user's existing settings**. Full design:
[DESIGN.md](DESIGN.md); founding pivot: [ADR 0001](docs/adr/0001-single-process-tui-embed-neovim.md).

karya has two internal layers, both shipped inside the one binary:
- a **headless workflow engine** (Go): tasks, specs, worktrees, gates, the agent
  adapter layer, git, and marketplaces — all testable without a terminal.
- a **single-process TUI runtime** (Go, stdlib-only): terminal I/O + cell buffer
  (`term`/`cellbuf`), the Elm-style `tui` runtime, the PTY host (`pty`), the
  Neovim RPC client (`nvimrpc`), the unified keymap engine (`keymap`), the
  window/pane manager (`layout`), and the views (`gitui`/`taskview`/`reviewview`/…).
- a slim embedded Neovim config (Lua) — **engine only**: options, LSP, treesitter,
  completion. No UI, statusline, or keymap plugins; karya owns all of that.

## Locked decisions (do not relitigate)
- **Single process; karya owns the terminal.** No external multiplexer (tmux
  gone), no external git TUI (lazygit gone). karya renders its own UI.
- **Neovim is embedded as an engine, not run as a UI.** Reuse Neovim for editing
  (LSP/treesitter/text ops) via `nvim --embed` msgpack-RPC; karya draws the grid
  and routes all input. Do not shell out to a user-facing `nvim`.
- **One leader, one keymap.** A single `keymap` engine (leader `Ctrl-Space`)
  drives every IDE action; unclaimed keys are forwarded to the focused pane. No
  per-tool keymap layers.
- **Go**, single static binary, no CGO. `go.mod` has **zero external
  dependencies** — stdlib only. The TUI stack (cell buffer, ANSI/terminfo, PTY,
  msgpack-RPC) is built from scratch to honor this. Adding a dependency is an
  ADR-level decision.
- **BYO agent CLIs** (`crush/claude/codex/gemini/aider/copilot`) as first-class,
  behind the `internal/agentrun` adapter interface; a native LLM agent may be
  added behind the same interface.

---

## The one rule that governs everything: isolation
karya must **never** read or write the user's `~/.zshrc`, `~/.tmux.conf`,
`~/.gitconfig`, `~/.config/nvim`, Homebrew, or global mise. All state lives under
karya-owned dirs. The primitives:
- Neovim: spawn `nvim --embed` with `NVIM_APPNAME=karya/nvim` (isolated
  config/data/state/cache; the `/nvim` suffix nests it below the karya prefix —
  use `config.NvimAppName`). karya connects over msgpack-RPC; it never launches a
  user-facing `nvim`.
- Panes: karya hosts shells and agent CLIs in its own PTYs (`internal/pty`), each
  spawned through the `karya shell` wrapper so the isolated env/prompt is applied
  without touching rc files. (No tmux server, no dedicated socket — removed by the
  single-process pivot.)
- Shell: **opt-in** only via `eval "$(karya shellenv)"`; never edit rc files.
- Tools: detect on `PATH` first; otherwise install into the karya prefix.

Every path goes through `internal/config` — never hardcode `~/.config/...`.
`karya uninstall` must remove everything karya created and nothing else. Any PR
that could violate isolation must include a test proving it does not. See
[DESIGN.md](DESIGN.md) §4 before touching install/launch code.

---

## Essential commands

```bash
make build                       # ./bin/karya (version injected via ldflags -X)
make gate                        # full pre-PR gate, mirrors CI exactly — run this
make test                        # go test ./...
go test -race ./...              # unit tests with race detector (as CI runs them)
go test -tags=integration ./...  # integration tests; needs nvim on PATH (+ a pty)
make lint                        # golangci-lint v2 via `go run ...@latest`
make fmt / make vet / make tidy  # format, static analysis, module tidy
make sync-docs                   # REQUIRED after editing docs/*.md (see docs section)
make formula TAG=vX.Y.Z SUMS=dist/checksums.txt   # regenerate Homebrew formula
```

Gotchas:
- Version info only appears when built with the ldflags in the Makefile or
  GoReleaser; a bare `go build` reports `dev`/`none`.
- Integration tests skip gracefully when `nvim` (or a usable pty) is missing, so
  a green local run without it does **not** prove integration behavior.

---

## Test-driven development (required)
We practice TDD. The loop is **Red → Green → Refactor**:

1. **Red** — write a test that expresses the desired behavior; run it; watch it fail.
2. **Green** — write the minimum code to make it pass.
3. **Refactor** — clean up (names, duplication, structure) with tests staying green.

Rules:
- No production behavior change lands without a test that fails before it and
  passes after it.
- **Unit tests** are hermetic: no network, no real tmux, no writes outside
  `t.TempDir()`. Put pure logic here (path resolution, agent selection, string
  escaping, cycling math, prefs parsing).
- **Integration tests** carry `//go:build integration` and drive real
  subsystems: the `pty` host on a real pseudo-terminal, `nvim --embed` over RPC,
  and real `git worktree` in `t.TempDir()` repos, asserting behavior then tearing
  down. They must never touch the user's dirs. **PTY TUI tests** run the real
  `karya` binary on a pty, script keystrokes, and scrape the screen (DESIGN.md
  §8.1). End-to-end HITL tests carry `-tags=e2e`.
- **TUI is tested like a backend.** Every UI component is a `tui.Model`
  (`Update`/`View` are pure). Test at four levels: model unit tests (feed `Msg`s,
  assert model+`Cmd`), `cellbuf` golden snapshots (`View` → buffer → golden,
  `-update` regenerates), PTY integration, and e2e. Impure boundaries have fakes:
  fake nvim (scripted RPC peer replaying `redraw` batches), fake PTY (in-memory
  pipes), fake agents (`agentrun.Runner`), fake terminal (`term.Output` →
  `bytes.Buffer`). Real nvim/pty go behind `-tags=integration`.
- **Editor-engine guardrail:** the slim embedded Neovim config keeps a headless
  RPC smoke test (open a buffer, confirm LSP attaches) plus the docs drift test.
  karya owns keymaps now, so there is no `<leader>c`-per-language Lua guardrail;
  keep `docs/keymaps.md` in sync when the unified `keymap` table changes.
- Design for testability: separate side effects from logic — workflow logic lives
  in headless packages, never in a `View`. If a behavior is only reachable through
  the UI, push it down. When something is hard to test, that's a design signal —
  refactor rather than skip the test.
- Prefer table-driven tests. Cover edge cases and error paths, not just happy paths.

---

## Design principles (SOLID + pragmatic Go)
Keep the codebase readable and maintainable. Apply SOLID with a Go accent:

- **Single Responsibility** — one package = one capability (`term`, `cellbuf`,
  `tui`, `pty`, `nvimrpc`, `keymap`, `layout`, `gitui`, `task`, `agentrun`,
  `config`, `tools`, `update`, `doctor`). Functions do one thing; if a function
  needs a paragraph to explain, split it. TUI layers depend only downward.
- **Open/Closed** — extend via new implementations, not by editing callers. The
  agent adapter layer is the prime example: adding an agent (or a native engine)
  means adding an implementation behind `agentrun.Agent`, not rewriting callers.
  Likewise `layout.PaneContent` (editor/terminal/view panes) is closed for
  modification, open for new pane kinds.
- **Liskov** — any implementation of an interface must be substitutable without
  surprising callers.
- **Interface Segregation** — small, focused interfaces defined by the *consumer*.
  Don't force a fat interface where a one-method one will do.
- **Dependency Inversion** — high-level orchestration (session/CLI) depends on
  abstractions (e.g. a tmux runner, an agent), not concrete globals. Pass
  dependencies in; avoid hidden package-level state. This is what makes TDD easy.

Also:
- **Composition over inheritance** (Go has no inheritance — embrace small structs
  and interfaces).
- **Accept interfaces, return structs.**
- **Errors are values**: wrap with `%w` and context; never `panic` for expected
  failures; surface actionable messages (point users at `karya doctor`).
- **Stdlib first**: add `github.com/spf13/cobra` only when the command tree
  justifies it; keep the dependency graph small. No CGO. (The CLI dispatcher in
  `internal/cli` is a hand-rolled stdlib `switch` — intentional.)
- **DRY, but not prematurely**: extract shared helpers once duplication is real.
- **Keep functions short and the public surface minimal** — unexport by default.

---

## Documentation standards
Good docs are part of "done":
- Every exported package, type, function, and method has a doc comment (Go style:
  start with the identifier name). `revive`'s `exported` rule enforces this in CI.
- Each package has a package comment explaining its responsibility and how it
  fits the isolation model.
- **Docs are split by audience — keep them on separate paths:**
  - **User-facing product docs** live in `docs/` (`tutorial.md`, `keymaps.md`,
    and the planned `commands.md`/`languages.md`/`isolation.md`). These are what
    ship embedded in the binary (Phase 7). Write them for a karya *user*.
  - **Internal engineering docs** live at the repo root (`DESIGN.md`,
    `ROADMAP.md`,
    `PROGRESS.md`, `AGENTS.md`). These are for contributors and are never shipped.
  - Never mix the two: user docs don't reference internal planning, and internal
    docs aren't embedded or surfaced to end users.
- **Docs are vendored for embedding.** `docs/*.md` is the authoritative source;
  `scripts/sync-docs.sh` (`make sync-docs`) copies them into
  `internal/assets/docs/`, which is go:embed'd. A drift test in
  `internal/assets` **fails CI** if you edit `docs/` without re-vendoring and
  committing the result.
- User-facing changes update `docs/` and the design record:
  - new/changed commands → `DESIGN.md` §12 and `README.md`;
  - new keymaps/workflows → `docs/tutorial.md` and `docs/keymaps.md`.
- Update `ROADMAP.md` (tick boxes) and `PROGRESS.md` (status + resume point +
  changelog entry) whenever a phase advances.
- Comments explain **why**, not what the code already says.

---

## Where things live
```
main.go                 entrypoint → internal/cli
internal/cli/           command dispatch + flags + app wiring; launches the TUI
internal/config/        XDG paths + karya prefix (isolation lives here)
internal/version/       version/build info
internal/assets/        go:embed payload (slim nvim engine config + shell init +
                        user docs) + extract/version

─ single-process TUI runtime (stdlib-only; new) ─
internal/term/          raw-mode terminal I/O, ANSI, terminfo-lite, input decoder
internal/cellbuf/       styled cell grid + minimal diff renderer (snapshot target)
internal/tui/           Elm-style Model/Update/View runtime + Program loop
internal/pty/           PTY host + pty/vt terminal emulator for shell/agent panes
internal/nvimrpc/       nvim --embed msgpack-RPC client + Grid model + msgpack codec
internal/keymap/        unified keymap engine (single leader, modal, which-key)
internal/layout/        window/pane/tab tree: splits, focus, resize, geometry
internal/gitui/         built-in git panel (replaces lazygit)
internal/taskview/  gateview/  reviewview/  diffview/   karya-native views

─ headless workflow engine (reused) ─
internal/spec/          task spec contract: parse/validate/render (DESIGN.md §3)
internal/task/          task engine: gate state machine + .karya/tasks/<id>/ store
internal/gate/          gate model, approvals, delegation, audit log
internal/review/        review-session assembly (plan/diff/evidence + feedback)
internal/worktree/      per-task git worktrees on task/<id> branches
internal/git/           headless git service (status/stage/commit/diff/log/push)
internal/agentrun/      Agent interface + adapters (BYO CLIs) + Runner/Git seam
internal/prompts/       step prompt assembly (spec + feedback + context)
internal/agent/         agent detection
internal/skills/  mcp/  marketplace clients (later phases)
internal/native/        built-in Claude-API agent engine (behind agentrun)
internal/project/  lang/  tools/  toolreg/  prefs/  doctor/  update/  tutorial/

docs/                   USER-FACING product docs (embedded in binary)
docs/adr/               architecture decision records (ADR 0001 = the pivot)
DESIGN.md ROADMAP.md PROGRESS.md AGENTS.md  INTERNAL docs (root; never shipped)
README.md CONTRIBUTING.md                  user landing + contributor entry
```

Notes:
- **Removed by the single-process pivot:** `internal/tmuxx/`, `internal/session/`,
  `internal/editor/`, `internal/assets/tmux.conf`, and the UI/keymap subtree of
  `internal/assets/nvim/`. If you find lingering references, they are cleanup.
- The embedded Neovim config in `internal/assets/nvim/` is now **engine-only**
  (options + LSP + treesitter + completion). karya sets chrome-off options
  (`laststatus=0`, no tabline/statusline) at runtime via RPC after attach.
- **Tool provisioning pins prebuilt backends.** In `internal/toolreg/catalog.go`,
  tools must resolve to prebuilt **aqua** backends (e.g.
  `aqua:neovim/neovim@0.11.7`) — bare mise names resolve to build-from-source
  plugins and break fresh installs. `GenerateMiseConfig` quotes TOML keys so
  backend-qualified keys stay valid.

## Command surface
| karya command | What it does |
|---|---|
| `karya` (default) | Launch the single-process TUI IDE for the cwd |
| `edit` | Open a file in the embedded editor pane; also karya's `$EDITOR` |
| `task` | Create/list/show/start/abandon tasks (the HITL unit of work) |
| `plan` / `implement` | Run an agent step headlessly in a task worktree |
| `review` / `gate` | Review artifacts; approve/reject/delegate gates |
| `verify` / `merge` | Run spec verification; merge or open a PR (post-gate) |
| `new` / `project` | Scaffold a new project for a supported language |
| `lang` | Choose languages + runtimes; regenerate the isolated mise config |
| `install` / `update` / `uninstall` | Isolated self-install, self-update, and teardown |
| `doctor` | Health checks: tools, versions, isolation, nvim-embed + pty |
| `help` / `docs` / `tutorial` | Offline docs embedded in the binary |

---

## Git & PR workflow
- **Always start from an up-to-date remote.** Before any new work, fetch and
  fast-forward `main` from the remote, then branch off it — never build on a
  stale local `main` or reuse an existing feature branch:
  ```bash
  git fetch origin
  git checkout main
  git pull --ff-only origin main
  git checkout -b feat/<slug>   # new branch per unit of work
  ```
- Branch from `main`: `feat/…`, `fix/…`, `docs/…`, `refactor/…`, `ci/…`,
  `test/…`, `chore/…`. Never commit directly to `main`.
- **Every unit of work gets its own fresh branch and its own PR** — open a PR for
  the change (`gh pr create`) rather than pushing to `main`.
- [Conventional Commits](https://www.conventionalcommits.org):
  `feat(agent): cycle to next detected agent`. Imperative, ≤72-char subject.
- One focused change per PR. Fill in the PR template. Link the issue.
- Keep `ROADMAP.md`/`PROGRESS.md` current in the same PR when a phase moves.

## Verify BEFORE opening a PR (mandatory gate)
Run the exact checks CI runs; all must be green. **Do not open a PR until they pass.**
`make gate` runs the whole sequence below; `make lint` runs just golangci-lint.

```bash
export PATH="/opt/homebrew/bin:$PATH"   # if Go is via Homebrew
gofmt -l .                       # must print nothing
go vet ./...                     # must pass
golangci-lint run                # must pass (golangci-lint v2; see below)
go test -race ./...              # unit tests (race-enabled)
go test -tags=integration ./...  # integration tests (requires nvim + a pty)
go build ./...                   # builds clean
```
CI installs **golangci-lint v2 at `latest`** (`.github/workflows/ci.yml` sets
`version: latest`; `.golangci.yml` is the v2 schema). A newer `latest` enables
stricter checks than any pinned version, so **run `@latest` locally to match CI**
— a version pin can pass locally and still fail the PR. If golangci-lint isn't
installed, run it without installing:
```bash
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest run ./...
```
CI (`.github/workflows/ci.yml`) enforces the same on Linux + macOS: `lint`,
`test`, `integration`, and cross-`build` jobs. Branch protection should require
these before merge. Releases are automated by GoReleaser on `v*` tags.

## Definition of done (per change)
1. Tests written first and passing (`-race` and, if tmux-related, `-tags=integration`).
2. `gofmt`, `go vet`, `golangci-lint`, `go build` all clean.
3. Exported symbols documented; user-facing docs updated (`make sync-docs` run).
4. **Isolation preserved** (and tested where relevant).
5. `ROADMAP.md` / `PROGRESS.md` updated if a phase advanced.
6. Conventional-commit messages; PR template completed; CI green.

## Maintaining karya as a healthy open-source project
- Triage issues with labels (`bug`, `enhancement`, `good first issue`, `docs`).
- Be responsive and kind in reviews; uphold [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
- Prefer small, reviewable PRs; require green CI + at least one review to merge.
- Keep `README`, `docs/tutorial.md`, and `docs/keymaps.md` in sync with behavior —
  the self-guided tutorial must always match what karya actually does.
- Security reports go through private advisories ([SECURITY.md](SECURITY.md)),
  never public issues.
- Keep dependencies current via Dependabot; keep the dependency set minimal.
