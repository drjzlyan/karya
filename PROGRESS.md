# karya — Progress Log

Living status document. **Read this first when resuming work.** It records what
is done, what is in flight, and the exact next action. Update it at the end of
every working session.

- **Plan:** [docs/PLAN.md](docs/PLAN.md)
- **Roadmap:** [ROADMAP.md](ROADMAP.md)
- **Agent/dev guide:** [AGENT.md](AGENT.md)

---

## Current status

**Active phase:** Phase 2 — Agent management (next)
**Overall:** Phases 0 and 1 complete. `karya` launches a fully isolated tmux IDE
session (editor/agent/build panes + git window); `karya edit`/`run` route into
panes; build + vet + tests are green. Go 1.26 is installed.

### Resume point (do this next — Phase 2)
1. `internal/prefs` — per-project `key=value` store at `paths.PrefsFile()`
   (port `load_pref`/`save_pref` from `ide-agent.sh`). Wire into `dev` agent
   resolution (flag → saved pref → single → picker) and save the choice.
2. `internal/agent` — add `switch/next/prev/reset` operating on `@ide_*` tmux
   options (faithful port of `ide-agent.sh`: respawn agent pane, rebuild layout
   preserving the editor pane).
3. Implement `karya agent switch|next|prev|reset|prefs|clear` in `internal/cli`
   (status already done). The tmux keybindings `Ctrl-a A/N/D` already call these.
4. Add a test for prefs round-trip and agent cycling index math.

### Verify the current build
```bash
export PATH="/opt/homebrew/bin:$PATH"
make build && go vet ./... && go test ./...
./bin/karya dev -a none <name> <dir>   # builds session on `tmux -L karya`
```

---

## Phase checklist (summary — details in ROADMAP.md)

| Phase | Title | Status |
|---|---|---|
| 0 | Scaffold & CLI skeleton | ☑ done |
| 1 | Session orchestration (`dev`) | ☑ done |
| 2 | Agent management | ◐ next |
| 3 | Editor integration (embedded nvim) | ☐ |
| 4 | Project scaffolding (`new`) | ☐ |
| 5 | Language & tool management | ☐ |
| 6 | Install / update / uninstall | ☐ |
| 7 | Doctor, docs, polish, distribution | ☐ |
| 8 | (Deferred) Native agent | ☐ |

---

## Changelog

### 2026-07-29 — Phase 0 + Phase 1 complete
- Installed Go 1.26; skeleton builds and runs (`karya version`, `--help`).
- **Phase 1 shipped:** `internal/{tmuxx,assets,agent,session,editor}` + CLI wiring.
  - `karya` / `karya dev [name] [path]` builds the tmux IDE session on the
    dedicated `-L karya` socket with embedded `tmux.conf` (`-f`); layout is
    editor(65%) | agent / build+test, plus a `git` window (lazygit).
  - Isolation verified live: session env carries `NVIM_APPNAME=karya` and
    `EDITOR/VISUAL/GIT_EDITOR=<karya> edit`; all `@ide_*` state options set; the
    user's **default tmux server is untouched**.
  - `karya edit <file> [line]` (port of `nvim-edit`) and `karya run [-d dir]
    <cmd>` / `--focus` (port of `ide-run`) route into the right panes, with
    direct-exec fallbacks outside tmux. `-k` recreate, `-q` quit implemented.
  - `karya agent status` detects installed agents.
  - Tests: isolation (paths/env), vim-escape/shell-quote, agent resolution.
    `go vet` + `go test ./...` green.
- Note: Go's `flag` needs options before positionals (`karya dev -a none foo`).

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
