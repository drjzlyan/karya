# karya

[![CI](https://github.com/drjzlyan/karya/actions/workflows/ci.yml/badge.svg)](https://github.com/drjzlyan/karya/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/drjzlyan/karya)](https://goreportcard.com/report/github.com/drjzlyan/karya)

**An AI-first, terminal-based IDE in a single binary.**

karya (कार्य — "work") turns Neovim, tmux, and your AI coding agent — Claude,
Codex, Crush, Gemini, aider, or Copilot — into one cohesive terminal IDE, with
LSP, debugging (DAP), git, and per-language tooling built in. It installs,
launches, manages, and updates the whole stack from a single self-contained Go
binary — and it does so **without touching any of your existing settings**.

> Status: **actively developed, pre-1.0.** Installable today (`v0.1.x` on macOS and
> Linux) — the core IDE, AI-agent management, project scaffolding, language
> tooling, and self-update all work. See [ROADMAP.md](ROADMAP.md) and
> [PROGRESS.md](PROGRESS.md).

## Why

A productive terminal IDE usually means hand-assembling a Neovim config, a tmux
layout, a pile of shell scripts, and an AI agent — then re-doing it on every new
machine. karya packages the whole workflow into one program, and promotes the AI
coding agent from an optional pane to a first-class, deeply-integrated feature.

## Features

- **AI coding agents, first-class.** Auto-detects and manages Claude, Codex,
  Crush, Gemini, aider, and Copilot in a dedicated pane — switch, cycle, and reset
  with per-project memory.
- **Full Neovim IDE, preconfigured.** LSP, autocompletion, debugging (DAP),
  Treesitter, and git (lazygit) work out of the box — no plugin wrangling.
- **6 languages, zero setup.** Python, Java, TypeScript, Go, C/C++, and Rust, with
  runtimes, LSP servers, and formatters installed into an isolated toolchain.
- **tmux IDE layout.** Editor, agent, and build/test panes plus a git window,
  driven by simple `karya` commands and keymaps.
- **Single Go binary, self-updating.** One checksum-verified, atomically
  self-replacing artifact; no runtime dependencies, no manual assembly.
- **Fully isolated & reversible.** Namespaced under karya-owned directories; never
  edits your shell rc, tmux, git, or Neovim config. `karya uninstall` leaves no
  trace.

## Zero-impact by design

karya's defining constraint: **installing it changes nothing you already have.**
It never edits your `~/.zshrc`, `~/.tmux.conf`, `~/.gitconfig`, or
`~/.config/nvim`. The editor runs under `NVIM_APPNAME=karya/nvim` and tmux on a
dedicated socket, so every config, plugin, tool, and cache lives under
karya-owned directories — isolated from your existing setup and your global
Homebrew/mise. `karya uninstall` removes karya and nothing else.

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
brew trust drjzlyan/karya   # Homebrew 6.0+ requires trusting non-official taps
brew install karya
```

> **Why `brew trust`?** Since Homebrew 6.0, loading a formula from a non-official
> tap is refused until you trust it (a tap can run its own Ruby code). Trusting the
> tap is a one-time step. To trust only this formula instead of the whole tap, use
> `brew trust --formula drjzlyan/karya/karya`.

Optionally wire karya into your shell (adds it to `PATH` and sets `$EDITOR`):

```bash
eval "$(karya shellenv)"
```

Keep it current with `karya update` (checksum-verified, atomic self-replace), and
remove it — and only it — with `karya uninstall`.

## Quick start

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
