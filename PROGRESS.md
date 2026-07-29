# karya — Progress Log

Living status document. **Read this first when resuming work.** It records what
is done, what is in flight, and the exact next action. Update it at the end of
every working session.

- **Plan:** [docs/PLAN.md](docs/PLAN.md)
- **Roadmap:** [ROADMAP.md](ROADMAP.md)
- **Agent/dev guide:** [AGENT.md](AGENT.md)

---

## Current status

**Active phase:** Phase 3 — Editor integration (embedded Neovim config) (next)
**Overall:** Phases 0–2 complete. `karya` launches a fully isolated tmux IDE
session (editor/agent/build panes + git window); `karya edit`/`run` route into
panes; agent detection/switching/cycling/reset + per-project memory are wired;
build + vet + golangci-lint + unit/race/integration tests are green. Go 1.26.

### Resume point (do this next — Phase 3)
1. Vendor `../nvim-config` into `assets/nvim/` (build step / sync script) and
   `go:embed` it; extract to `paths.NvimConfig()` (`~/.config/karya/nvim`).
2. Version the extracted tree via a `manifest.json` so `update` re-extracts only
   when the embedded config changed.
3. Launch Neovim with `NVIM_APPNAME=karya` (already set in session env) and
   isolated data/state/cache dirs; bootstrap plugins headless
   (`nvim --headless +"Lazy! sync" +qa`).
4. Add an isolation test proving the user's `~/.config/nvim` is never read/written.

### Phase 2 — what shipped
- `internal/prefs` — flat `key=value` store at `paths.PrefsFile()` (port of
  `load_pref`/`save_pref`); Get/Set/Delete/Entries, order-preserving. Wired into
  `dev` resolution: flag → saved pref → single → picker, and the choice is saved.
- `internal/agent.Manager` — in-session `Next/Prev/SwitchTo/SwitchInteractive/
  Reset/ClearPref/StatusText` over `@ide_*` options; consumer-defined
  `TmuxRunner`/`PrefStore` interfaces keep unit tests hermetic (fake tmux + fake
  prefs). Faithful port of `ide-agent.sh` respawn/layout-reset semantics.
- `karya agent switch|switch-to|next|prev|reset|status|prefs|clear` in the CLI;
  `Ctrl-a A/N/D` keybindings drive them. `switch` opens a tmux command-prompt
  that calls back `karya agent switch-to %%`.

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
| 2 | Agent management | ☑ done |
| 3 | Editor integration (embedded nvim) | ◐ next |
| 4 | Project scaffolding (`new`) | ☐ |
| 5 | Language & tool management | ☐ |
| 6 | Install / update / uninstall | ☐ |
| 7 | Embedded help, self-guided tutorial, doctor & distribution | ☐ |
| 8 | (Deferred) Native agent | ☐ |

---

## Changelog

### 2026-07-30 — Phase 2 complete: agent management + per-project memory
- **`internal/prefs`**: flat `key=value` store (port of `load_pref`/`save_pref`)
  with `Get/Set/Delete/Entries`, order-preserving replace, lazy file creation.
  Round-trip/replace/delete/value-with-`=` unit tests, all under `t.TempDir()`.
- **`internal/agent.Manager`**: in-session `Next/Prev/SwitchTo/SwitchInteractive/
  Reset/ClearPref/StatusText` operating on `@ide_*` tmux options; faithful port
  of `ide-agent.sh` (respawn agent pane, rebuild right column, recreate the dev
  window if lost, relaunch current agent). Depends on consumer-defined
  `TmuxRunner`/`PrefStore` interfaces (dependency inversion) so cycling math and
  state transitions are unit-tested with a fake tmux + fake prefs (hermetic).
- **CLI**: `karya agent switch|switch-to|next|prev|reset|status|prefs|clear`;
  `dev` resolution now layers the saved per-project preference (flag → pref →
  single → picker) and persists the choice. `Ctrl-a A/N/D` already bound.
- Gate green: `gofmt`, `go vet`, `golangci-lint`, `go test -race`,
  `go test -tags=integration`, `go build`.

### 2026-07-30 — Open-source setup: CI, governance, docs, tutorial
- **CI** (`.github/workflows/ci.yml`): `lint` (gofmt/vet/golangci-lint),
  `test` (race+coverage on Linux+macOS), `integration` (installs tmux, runs
  `-tags=integration`), and cross-`build` (darwin/linux × amd64/arm64). Merges
  gated on green CI. Added `release.yml` (GoReleaser on `v*`), `.goreleaser.yaml`,
  `.golangci.yml`, and `.github/dependabot.yml`.
- **SOLID refactor:** split `session.Build` (testable, no attach) from
  `session.Dev` (Build+Attach); added integration tests asserting layout, env,
  `@ide_*` state, and default-server isolation.
- **Governance/OSS files:** `LICENSE` (MIT), `CONTRIBUTING.md`,
  `CODE_OF_CONDUCT.md`, `SECURITY.md`, PR template, issue templates.
- **AGENT.md** rewritten as the engineering guide: mandatory TDD (Red-Green-
  Refactor), SOLID + Go design principles, documentation standards, git/PR
  conventions, and a **pre-PR verification gate** (the exact CI checks).
- **Docs:** `docs/tutorial.md` (self-guided: tmux/nvim fundamentals, agents, and
  Python/Java/TS/Go/C++/Rust walkthroughs) and `docs/keymaps.md` (full CLI/tmux/
  nvim reference). README: badges, license, contributing section.
- Repo published: remote `git@github.com:drjzlyan/karya.git`, public.

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
