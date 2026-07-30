# karya

[![CI](https://github.com/drjzlyan/karya/actions/workflows/ci.yml/badge.svg)](https://github.com/drjzlyan/karya/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/drjzlyan/karya)](https://goreportcard.com/report/github.com/drjzlyan/karya)

**An AI-first, terminal-based IDE in a single binary.**

karya (कार्य — "work") orchestrates Neovim, tmux, and your coding agent into one
cohesive terminal IDE. It installs, launches, manages, and updates the whole
stack from a single self-contained Go binary — and it does so **without touching
any of your existing settings**.

> Status: **early development.** Planning is complete and the CLI skeleton
> builds; features are being implemented phase by phase. See
> [ROADMAP.md](ROADMAP.md) and [PROGRESS.md](PROGRESS.md).

## Why

A productive terminal IDE usually means hand-assembling a Neovim config, a tmux
layout, a pile of shell scripts, and an AI agent — then re-doing it on every new
machine. karya packages the whole workflow into one program, and promotes the AI
coding agent from an optional pane to a first-class, deeply-integrated feature.

## Principles

- **Single binary.** The Neovim and tmux configs are embedded and extracted on
  demand — one artifact, no manual assembly.
- **Zero-impact install.** karya never edits your `~/.zshrc`, `~/.tmux.conf`,
  `~/.gitconfig`, or `~/.config/nvim`. It uses `NVIM_APPNAME=karya/nvim` and a
  dedicated tmux socket, keeping everything under karya-owned directories.
  `karya uninstall` removes karya and nothing else.
- **AI-first.** Detects and manages your existing agent CLIs (claude, codex,
  crush, gemini, aider, copilot) as a core pane with per-project memory.
- **Self-updating.** `karya update` upgrades the binary, configs, tools, and
  editor plugins.

## Install

The universal installer works the same on macOS and Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/drjzlyan/karya/main/scripts/install.sh | sh
```

This downloads the right prebuilt binary from GitHub Releases, verifies its
SHA-256 against the published checksums, installs it to `~/.local/bin/karya`, and
runs the isolated `karya install` setup — **without touching your shell rc,
Homebrew, or global mise**.

Or via Homebrew (macOS and Linux):

```bash
brew tap drjzlyan/karya https://github.com/drjzlyan/karya
brew install karya
```

Optionally wire karya into your shell (adds it to `PATH` and sets `$EDITOR`):

```bash
eval "$(karya shellenv)"
```

Keep it current with `karya update` (checksum-verified, atomic self-replace), and
remove it — and only it — with `karya uninstall`.

## Quick start (target UX)

```bash
karya                      # launch/attach the IDE for the current directory
karya new go myapp         # scaffold a project
karya agent next           # cycle the coding agent
karya doctor               # health check
karya update               # self-update
```

## Build from source

```bash
brew install go            # Go toolchain required
make build                 # -> ./bin/karya
./bin/karya version
```

## Documentation

**For users** (product docs, in `docs/`):

- [docs/tutorial.md](docs/tutorial.md) — **self-guided tutorial**: learn every
  keymap, movement, and per-language workflow hands-on
- [docs/keymaps.md](docs/keymaps.md) — full CLI / tmux / Neovim key reference

**For contributors** (internal engineering docs, at the repo root):

- [PLAN.md](PLAN.md) — architecture, isolation model, subsystems
- [ROADMAP.md](ROADMAP.md) — phased build order
- [PROGRESS.md](PROGRESS.md) — current status / resume point
- [AGENT.md](AGENT.md) — engineering guide (TDD, SOLID, CI, isolation)

## Contributing

Contributions are welcome! Start with [CONTRIBUTING.md](CONTRIBUTING.md) and the
engineering guide in [AGENT.md](AGENT.md). We use **test-driven development**,
SOLID design, and a green-CI-before-merge policy. Please also read our
[Code of Conduct](CODE_OF_CONDUCT.md). Security issues: see [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE) © 2026 Dhiraj Salian and karya contributors.
