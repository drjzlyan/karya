# AGENT.md — Guide for resuming karya development

This file orients an AI agent (or a human) picking up karya. Read it, then read
[PROGRESS.md](PROGRESS.md) for the exact resume point.

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

## The one rule that governs everything: isolation
karya must **never** read or write the user's `~/.zshrc`, `~/.tmux.conf`,
`~/.gitconfig`, `~/.config/nvim`, Homebrew, or global mise. All state lives under
karya-owned dirs. The primitives:
- Neovim: launch with `NVIM_APPNAME=karya` (isolated config/data/state/cache).
- tmux: run on a dedicated socket `tmux -L karya -f <karya tmux.conf>`.
- Shell: **opt-in** only via `eval "$(karya shellenv)"`; never edit rc files.
- Tools: detect on `PATH` first; otherwise install into the karya prefix.
`karya uninstall` must be able to remove everything karya created and nothing else.
See [docs/PLAN.md](docs/PLAN.md) §2 before changing any install/launch code.

## Where things live
```
main.go                 entrypoint → internal/cli
internal/cli/           command dispatch + flags
internal/config/        XDG paths + karya prefix (isolation lives here)
internal/version/       version/build info
internal/tmuxx/         tmux wrapper (dedicated socket)      [Phase 1]
internal/session/       `dev` layout + panes + git window    [Phase 1]
internal/agent/         detect/switch/cycle/prefs            [Phase 2]
internal/editor/        nvim launch + `edit` routing         [Phase 1/3]
internal/project/       `new` scaffolds                      [Phase 4]
internal/lang/          language/version selection           [Phase 5]
internal/tools/         tool detect/install                  [Phase 5]
internal/prefs/         per-project preference store         [Phase 2]
internal/doctor/        health checks                        [Phase 7]
internal/update/        self-update                          [Phase 6]
assets/                 go:embed payload (nvim config, tmux.conf, …)
docs/ ROADMAP.md PROGRESS.md README.md
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

## Conventions
- **Stdlib first.** Add `github.com/spf13/cobra` only when the command tree
  justifies it (Phase 1+). Keep deps minimal; no CGO.
- All paths go through `internal/config` — never hardcode `~/.config/...`.
- Every subprocess (nvim/tmux/git/lazygit) is spawned with karya's isolated env.
- Match the existing scripts' UX (flags, prompts, messages) so muscle memory
  carries over.
- Update **PROGRESS.md** at the end of each session (status + resume point +
  changelog entry). Tick boxes in **ROADMAP.md** as phases complete.

## Build / run / verify
```bash
brew install go          # toolchain not yet present on this machine
go build ./... && ./karya version
go vet ./... && go test ./...
make build               # once Makefile targets are filled in
```
There is no CI wired yet (Phase 0 task). Do not assume network access during the
first build — Phase 0 is stdlib-only for exactly this reason.

## Definition of done for a phase
The binary still builds and runs, the phase's ROADMAP boxes are ticked,
PROGRESS.md is updated, and isolation is preserved (add/keep an isolation test
that asserts no user config path is touched).
