# AGENT.md — Engineering guide for karya

This is the authoritative guide for anyone — human or AI — working on karya. Read
it fully before writing code, then read [PROGRESS.md](PROGRESS.md) for the exact
resume point. It defines **how** we build karya so the project stays correct,
maintainable, well-tested, well-documented, and welcoming to contributors.

## What karya is
An **AI-first, terminal-based IDE** shipped as a **single Go binary**. It
orchestrates Neovim (editor), tmux (multiplexer), and a coding agent into one
cohesive IDE, and installs/updates/uninstalls itself **without touching any of
the user's existing settings**. Full design: [docs/PLAN.md](docs/PLAN.md).

It consolidates two existing repos (kept as the behavioral source of truth):
- `../nvim-config` — the Neovim editor config (Lua). Gets embedded into karya.
- `../dotfiles` — the shell scripts / session tooling. Reimplemented in Go.

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
- Neovim: launch with `NVIM_APPNAME=karya` (isolated config/data/state/cache).
- tmux: run on a dedicated socket `tmux -L karya -f <karya tmux.conf>`.
- Shell: **opt-in** only via `eval "$(karya shellenv)"`; never edit rc files.
- Tools: detect on `PATH` first; otherwise install into the karya prefix.

Every path goes through `internal/config` — never hardcode `~/.config/...`.
`karya uninstall` must remove everything karya created and nothing else. Any PR
that could violate isolation must include a test proving it does not. See
[docs/PLAN.md](docs/PLAN.md) §2 before touching install/launch code.

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
- User-facing changes update `docs/`:
  - new/changed commands → `docs/PLAN.md` §4 and `README.md`;
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
internal/assets/        go:embed payload (tmux.conf, later nvim config) + extract
internal/tmuxx/         tmux wrapper (dedicated socket)
internal/session/       `dev` layout: Build (testable) + Dev (Build+Attach), Quit
internal/agent/         detect/select (switch/next/prev/reset land in Phase 2)
internal/editor/        `edit` (nvim-edit) + `run` (ide-run) routing
internal/project/       `new` scaffolds                      [Phase 4]
internal/lang/          language/version selection           [Phase 5]
internal/tools/         tool detect/install                  [Phase 5]
internal/prefs/         per-project preference store         [Phase 2]
internal/doctor/        health checks                        [Phase 7]
internal/update/        self-update                          [Phase 6]
docs/ ROADMAP.md PROGRESS.md README.md CONTRIBUTING.md
```

## Behavioral source of truth (port faithfully)
| karya piece | Port from |
|---|---|
| `session` / `dev` | `dotfiles/scripts/dev.sh` |
| `agent` | `dotfiles/scripts/ide-agent.sh` |
| `run` | `dotfiles/scripts/ide-run.sh` |
| `edit` | `dotfiles/bin/nvim-edit` |
| `project`/`new` | `dotfiles/scripts/project-init.sh` |
| `lang` | `dotfiles/scripts/languages.sh` |
| `install`/`update` | `dotfiles/{install,update,rebuild,link,doctor}.sh` |
| embedded editor | `nvim-config/**` |

---

## Git & PR workflow
- Branch from `main`: `feat/…`, `fix/…`, `docs/…`, `refactor/…`, `ci/…`,
  `test/…`, `chore/…`. Never commit directly to `main`.
- [Conventional Commits](https://www.conventionalcommits.org):
  `feat(agent): cycle to next detected agent`. Imperative, ≤72-char subject.
- One focused change per PR. Fill in the PR template. Link the issue.
- Keep `ROADMAP.md`/`PROGRESS.md` current in the same PR when a phase moves.

## Verify BEFORE opening a PR (mandatory gate)
Run the exact checks CI runs; all must be green. **Do not open a PR until they pass.**

```bash
export PATH="/opt/homebrew/bin:$PATH"   # if Go is via Homebrew
gofmt -l .                       # must print nothing
go vet ./...                     # must pass
golangci-lint run                # must pass (golangci-lint v2; see below)
go test -race ./...              # unit tests (race-enabled)
go test -tags=integration ./...  # integration tests (requires tmux)
go build ./...                   # builds clean
```
CI installs **golangci-lint v2** (`.golangci.yml` is the v2 schema). If it isn't
installed locally, run it without installing — pin the v2 module path so it
matches CI, not the old v1:
```bash
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run
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
