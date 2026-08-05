# AGENT.md — Engineering guide for karya

This is the authoritative guide for anyone — human or AI — working on karya. Read
it fully before writing code, then read [PROGRESS.md](PROGRESS.md) for the exact
resume point. It defines **how** we build karya so the project stays correct,
maintainable, well-tested, well-documented, and welcoming to contributors.

## What karya is
An **AI-first, terminal-based IDE** shipped as a **single Go binary**. It
orchestrates Neovim (editor), tmux (multiplexer), and a coding agent into one
cohesive IDE, and installs/updates/uninstalls itself **without touching any of
the user's existing settings**. Full design: [PLAN.md](PLAN.md).

karya has two internal layers, both shipped inside the one binary:
- an embedded Neovim editor configuration (Lua), extracted to the isolated karya
  prefix on install.
- Go-native session, agent, project, and tooling logic that drives Neovim, tmux,
  and the chosen coding agent.

## Locked decisions (do not relitigate)
- **Orchestrator, not a from-scratch editor.** Neovim + tmux are reused.
- **Go**, single static binary, no CGO.
- **BYO agent CLIs** (`crush/claude/codex/gemini/aider/copilot`) as first-class.
  A native LLM agent is deferred to Phase 8 but the interface must allow it.

---

## The one rule that governs everything: isolation
karya must **never** read or write the user's `~/.zshrc`, `~/.tmux.conf`,
`~/.gitconfig`, `~/.config/nvim`, Homebrew, or global mise. All state lives under
karya-owned dirs. The primitives:
- Neovim: launch with `NVIM_APPNAME=karya/nvim` (isolated config/data/state/cache;
  the `/nvim` suffix nests it below the karya prefix — use `config.NvimAppName`).
- tmux: run on a dedicated socket `tmux -L karya -f <karya tmux.conf>`.
- Shell: **opt-in** only via `eval "$(karya shellenv)"`; never edit rc files.
- Tools: detect on `PATH` first; otherwise install into the karya prefix.

Every path goes through `internal/config` — never hardcode `~/.config/...`.
`karya uninstall` must remove everything karya created and nothing else. Any PR
that could violate isolation must include a test proving it does not. See
[PLAN.md](PLAN.md) §2 before touching install/launch code.

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
- **Integration tests** carry `//go:build integration` and drive the real `tmux`
  binary on a **throwaway socket** (`tmuxx.New("karya-itest-…", …)`), asserting
  layout/env/state, then kill the server. They must never touch the user's tmux
  or the real karya dirs. Example: `internal/session/session_integration_test.go`.
- Design for testability: separate side effects from logic. (E.g. `session.Build`
  creates the layout with no `attach`, so it is testable; `session.Dev` composes
  `Build` + `Attach`.) When something is hard to test, that's a design signal —
  refactor rather than skip the test.
- Prefer table-driven tests. Cover edge cases and error paths, not just happy paths.

---

## Design principles (SOLID + pragmatic Go)
Keep the codebase readable and maintainable. Apply SOLID with a Go accent:

- **Single Responsibility** — one package = one capability (`session`, `agent`,
  `editor`, `tmuxx`, `config`, `prefs`, `tools`, `update`, `doctor`). Functions
  do one thing; if a function needs a paragraph to explain, split it.
- **Open/Closed** — extend via new implementations, not by editing callers. The
  agent layer is the prime example: adding a native agent (Phase 8) must mean
  adding an implementation behind an interface, not rewriting `agent`.
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
  justifies it; keep the dependency graph small. No CGO.
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
  - **Internal engineering docs** live at the repo root (`PLAN.md`, `ROADMAP.md`,
    `PROGRESS.md`, `AGENT.md`). These are for contributors and are never shipped.
  - Never mix the two: user docs don't reference internal planning, and internal
    docs aren't embedded or surfaced to end users.
- User-facing changes update `docs/` and the design record:
  - new/changed commands → `PLAN.md` §4 and `README.md`;
  - new keymaps/workflows → `docs/tutorial.md` and `docs/keymaps.md`.
- Update `ROADMAP.md` (tick boxes) and `PROGRESS.md` (status + resume point +
  changelog entry) whenever a phase advances.
- Comments explain **why**, not what the code already says.

---

## Where things live
```
main.go                 entrypoint → internal/cli
internal/cli/           command dispatch + flags + app wiring
internal/config/        XDG paths + karya prefix (isolation lives here)
internal/version/       version/build info
internal/assets/        go:embed payload (tmux.conf + vendored nvim config) + extract/version
internal/tmuxx/         tmux wrapper (dedicated socket)
internal/session/       `dev` layout: Build (testable) + Dev (Build+Attach), Quit
internal/agent/         detect/select/switch/send + Runner interface (pluggable engine)
internal/task/          Task model + per-project store         [Phase 10]
internal/worktree/      git worktree-per-task isolation        [Phase 10]
internal/editor/        `edit` (editor pane) + `run` (build/test pane) routing
internal/project/       `new` scaffolds                      [Phase 4]
internal/lang/          language/version selection           [Phase 5]
internal/tools/         tool detect/install                  [Phase 5]
internal/prefs/         per-project preference store         [Phase 2]
internal/doctor/        health checks                        [Phase 7]
internal/update/        self-update                          [Phase 6]

docs/                   USER-FACING product docs (embedded in binary, Phase 7)
PLAN.md ROADMAP.md PROGRESS.md AGENT.md    INTERNAL docs (root; never shipped)
README.md CONTRIBUTING.md                  user landing + contributor entry
```

## Command surface
| karya command | What it does |
|---|---|
| `dev` (default) | Build/attach the isolated tmux IDE session for the cwd |
| `agent` | Switch/cycle/reset the coding-agent pane in a session |
| `task` | Create/list/switch/remove tasks, each in an isolated worktree (branch `karya/<id>`) |
| `run` | Send a command to the build/test pane (or run it directly) |
| `edit` | Open a file in the editor pane; also karya's `$EDITOR` |
| `new` / `project` | Scaffold a new project for a supported language |
| `lang` | Choose languages + runtimes; regenerate the isolated mise config |
| `install` / `update` / `uninstall` | Isolated self-install, self-update, and teardown |
| `doctor` | Health checks: tools, versions, isolation, per-language tooling |
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
go test -tags=integration ./...  # integration tests (requires tmux)
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
3. Exported symbols documented; user-facing docs updated.
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
