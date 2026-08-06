# Contributing to karya

Thanks for your interest in improving karya — an AI-first, terminal-based IDE in
a single Go binary. This guide covers how we work so contributions stay
consistent, tested, and easy to review.

Please also read [AGENTS.md](AGENTS.md); it is the authoritative engineering guide
(architecture, isolation rules, TDD, SOLID, and the pre-PR checklist) for both
human and AI contributors. This document is the human-facing summary.

## Code of conduct

This project follows the [Contributor Covenant](CODE_OF_CONDUCT.md). By
participating you agree to uphold it.

## Ground rules

1. **The isolation guarantee is non-negotiable.** karya must never read or write
   the user's `~/.zshrc`, `~/.tmux.conf`, `~/.gitconfig`, `~/.config/nvim`,
   Homebrew, or global mise. Every path goes through `internal/config`. See
   [DESIGN.md](DESIGN.md) §4.
2. **Test-driven development.** Write a failing test first, then the code to make
   it pass, then refactor. No behavior change lands without a test.
3. **Keep it maintainable.** Follow SOLID and the design guidance in AGENTS.md.
   Prefer the standard library; add dependencies only when clearly justified.
4. **Document as you go.** Exported symbols get doc comments; user-facing changes
   update `docs/`; phase progress updates `ROADMAP.md` / `PROGRESS.md`.

## Getting started

```bash
# Prerequisites: Go (see go.mod for the version), tmux, git. nvim recommended.
git clone git@github.com:drjzlyan/karya.git
cd karya
make build          # -> ./bin/karya
./bin/karya version
```

## Development workflow

1. **Find or open an issue** describing the change. For anything non-trivial,
   discuss the approach first.
2. **Branch** from `main`:
   `git switch -c feat/<short-name>` (or `fix/`, `docs/`, `refactor/`, `ci/`).
3. **TDD loop**: add a failing test → implement → refactor → repeat.
4. **Run the full local gate** (see below) until green.
5. **Open a PR** using the template. Keep PRs focused and reviewable.

## Local verification (run before every PR)

```bash
gofmt -l .                       # must print nothing
go vet ./...                     # must pass
golangci-lint run                # must pass (install: https://golangci-lint.run)
go test ./...                    # unit tests
go test -tags=integration ./...  # integration tests (requires tmux)
```

CI runs the same checks on Linux and macOS; PRs cannot merge until they pass.

## Tests

- **Unit tests** are hermetic — no network, no real tmux, no writes outside a
  `t.TempDir()`. Pure functions and logic live here.
- **Integration tests** carry `//go:build integration` and exercise the real
  `tmux` binary on a throwaway server socket. They must never touch the user's
  default tmux server or real karya directories.

## Commit messages

Use [Conventional Commits](https://www.conventionalcommits.org):

```
feat(agent): cycle to the next detected agent
fix(session): reset layout when the agent pane is gone
docs(tutorial): add the Rust walkthrough
test(editor): cover vim filename escaping
refactor(session): split Build from Attach for testability
```

Keep the subject imperative and ≤ 72 characters. Explain the *why* in the body.

## Coding style

- `gofmt` / `goimports` formatting is enforced.
- Small, single-responsibility packages and functions.
- Return errors, don't panic (except truly unrecoverable startup states).
- Name things for what they do; match the surrounding code's idioms.
- Every subprocess (nvim, tmux, git, agents) is spawned with karya's isolated
  environment — never the ambient one.

## Documentation

Docs are split by audience: **user-facing product docs** live in `docs/` (and
ship embedded in the binary), while **internal engineering docs** (`DESIGN.md`,
`ROADMAP.md`, `PROGRESS.md`, `AGENTS.md`) live at the repo root. Keep them on
their separate paths — don't mix planning notes into user docs or vice versa.

- Package- and function-level doc comments on all exported symbols.
- User-facing features are documented in `docs/` and, where they involve
  keymaps or workflows, added to `docs/tutorial.md` and `docs/keymaps.md`.
- If you add or change a command, update `DESIGN.md` §12 and the README.

## Reporting bugs & requesting features

Use the issue templates. Include `karya version`, OS/arch, and exact commands.
Security issues: **do not** open a public issue — see [SECURITY.md](SECURITY.md).

## Releases

Releases are automated by GoReleaser on `v*` tags (see
`.github/workflows/release.yml`). Maintainers tag from `main` after CI is green.

Thank you for contributing! 🙌
