<!-- Thanks for contributing to karya! Please complete this checklist. -->

## Summary

<!-- What does this PR do and why? Link any related issue: Closes #123 -->

## Type of change

- [ ] Bug fix
- [ ] New feature
- [ ] Refactor / cleanup
- [ ] Documentation
- [ ] CI / tooling

## Which phase / area

<!-- e.g. Phase 2 — agent management; internal/session -->

## Checklist

- [ ] I followed **test-driven development**: tests were written/updated first and they fail without this change.
- [ ] `gofmt -l .` is clean and `go vet ./...` passes.
- [ ] `go test ./...` (unit) passes locally.
- [ ] `go test -tags=integration ./...` passes locally (tmux installed).
- [ ] `golangci-lint run` passes locally.
- [ ] Code follows the SOLID / design guidelines in [AGENTS.md](../AGENTS.md).
- [ ] Public functions/packages have doc comments; user-facing behavior is documented in `docs/`.
- [ ] **Isolation preserved**: no code reads or writes the user's `~/.zshrc`, `~/.tmux.conf`, `~/.config/nvim`, Homebrew, or global mise (all paths go through `internal/config`).
- [ ] Updated `ROADMAP.md` / `PROGRESS.md` if this completes or advances a phase.

## How was this tested?

<!-- Commands you ran; manual verification steps; screenshots if UI/TUI. -->
