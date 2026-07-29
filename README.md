# karya

**An AI-first, terminal-based IDE in a single binary.**

karya (कार्य — "work") orchestrates Neovim, tmux, and your coding agent into one
cohesive terminal IDE. It installs, launches, manages, and updates the whole
stack from a single self-contained Go binary — and it does so **without touching
any of your existing settings**.

> Status: **early development.** Planning is complete and the CLI skeleton
> builds; features are being implemented phase by phase. See
> [ROADMAP.md](ROADMAP.md) and [PROGRESS.md](PROGRESS.md).

## Why

The author's terminal IDE was assembled from a Neovim config, a tmux layout, and
a dozen shell scripts (`dev.sh`, `ide-agent.sh`, `project-init.sh`, `install.sh`,
…). karya consolidates all of it into one program, and promotes the AI coding
agent from an optional pane to a first-class, deeply-integrated feature.

## Principles

- **Single binary.** The Neovim and tmux configs are embedded and extracted on
  demand — one artifact, no manual assembly.
- **Zero-impact install.** karya never edits your `~/.zshrc`, `~/.tmux.conf`,
  `~/.gitconfig`, or `~/.config/nvim`. It uses `NVIM_APPNAME=karya` and a
  dedicated tmux socket, keeping everything under karya-owned directories.
  `karya uninstall` removes karya and nothing else.
- **AI-first.** Detects and manages your existing agent CLIs (claude, codex,
  crush, gemini, aider, copilot) as a core pane with per-project memory.
- **Self-updating.** `karya update` upgrades the binary, configs, tools, and
  editor plugins.

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

- [docs/PLAN.md](docs/PLAN.md) — architecture, isolation model, subsystems
- [ROADMAP.md](ROADMAP.md) — phased build order
- [PROGRESS.md](PROGRESS.md) — current status / resume point
- [AGENT.md](AGENT.md) — guide for resuming development

## License

TBD.
