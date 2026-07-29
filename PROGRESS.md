# karya — Progress Log

Living status document. **Read this first when resuming work.** It records what
is done, what is in flight, and the exact next action. Update it at the end of
every working session.

- **Plan:** [docs/PLAN.md](docs/PLAN.md)
- **Roadmap:** [ROADMAP.md](ROADMAP.md)
- **Agent/dev guide:** [AGENT.md](AGENT.md)

---

## Current status

**Active phase:** Phase 0 — Scaffold & CLI skeleton
**Overall:** Planning complete; repo scaffolded; command tree stubbed.

### Resume point (do this next)
1. Install Go (`brew install go`) — **not yet installed on this machine**.
2. From the repo root: `go build ./...` then `./karya version` to confirm the
   skeleton builds and runs.
3. Run `go vet ./...` and wire the GitHub Actions CI workflow.
4. Add `goreleaser` config (`.goreleaser.yaml`) for cross-compiled releases.
5. Begin **Phase 1** (`internal/tmuxx` + `internal/session`) per ROADMAP.

---

## Phase checklist (summary — details in ROADMAP.md)

| Phase | Title | Status |
|---|---|---|
| 0 | Scaffold & CLI skeleton | ◐ in progress |
| 1 | Session orchestration (`dev`) | ☐ |
| 2 | Agent management | ☐ |
| 3 | Editor integration (embedded nvim) | ☐ |
| 4 | Project scaffolding (`new`) | ☐ |
| 5 | Language & tool management | ☐ |
| 6 | Install / update / uninstall | ☐ |
| 7 | Doctor, docs, polish, distribution | ☐ |
| 8 | (Deferred) Native agent | ☐ |

---

## Changelog

### 2026-07-29 — Project inception
- Explored source repos `nvim-config` and `dotfiles`; mapped every feature.
- **Decisions locked in** (via clarifying questions):
  - Architecture: **orchestrator/launcher** binary (Neovim stays the editor).
  - Language: **Go** (single static binary, easy cross-compile & self-update).
  - AI agent: **BYO agent CLI, first-class** (detect/manage existing agent CLIs;
    native API agent deferred to Phase 8).
- Wrote [docs/PLAN.md](docs/PLAN.md) (full architecture + isolation model),
  [ROADMAP.md](ROADMAP.md), this file, and [AGENT.md](AGENT.md).
- Scaffolded repo: `go.mod`, `main.go`, `internal/{cli,config,version}`,
  `Makefile`, `.gitignore`, `assets/`. Every PLAN §4 command exists as a stub.
- **Blocker:** Go toolchain not installed — cannot `go build`/verify yet.

---

## Notes for the next session
- The **isolation model** (PLAN §2) is the defining constraint: never touch the
  user's `~/.zshrc`, `~/.tmux.conf`, `~/.config/nvim`, Homebrew, or global mise.
  Everything lives under the karya prefix; `NVIM_APPNAME=karya` + `tmux -L karya`
  are the isolation primitives.
- Source-of-truth behavior to port lives in:
  `dotfiles/scripts/dev.sh`, `ide-agent.sh`, `ide-run.sh`, `project-init.sh`,
  `languages.sh`; `dotfiles/{install,update,rebuild,link,doctor}.sh`;
  `dotfiles/bin/nvim-edit`; and the whole `nvim-config/` tree.
- Keep the dependency list minimal (stdlib → add `cobra` only when the tree
  grows). No CGO.
