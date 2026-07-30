# karya — Progress Log

Living status document. **Read this first when resuming work.** It records what
is done, what is in flight, and the exact next action. Update it at the end of
every working session.

- **Plan:** [PLAN.md](PLAN.md)
- **Roadmap:** [ROADMAP.md](ROADMAP.md)
- **Agent/dev guide:** [AGENT.md](AGENT.md)

---

## Current status

**Active phase:** Phase 7 — Embedded help, tutorial, doctor & distribution (in progress)
**Overall:** Phases 0–6 complete. `karya` launches a fully isolated tmux IDE
session (editor/agent/build panes + git window); `karya edit`/`run` route into
panes; agent detection/switching/cycling/reset + per-project memory are wired;
the full Neovim config ships embedded and extracts to a karya-namespaced dir
with zero impact on the user's own Neovim; `karya new` scaffolds all six
languages and opens the project in an IDE session; `karya lang` selects languages
+ runtime versions and installs their LSP/formatter/adapter tooling into the
karya prefix (isolated mise + tool bin). Full lifecycle is live: `karya install`
(isolated setup), `karya update [--check]` (checksum-verified atomic self-replace
from GitHub Releases), `karya uninstall` (karya-only removal), `karya shellenv`
(opt-in), plus a `curl | sh` installer. Build + vet + golangci-lint +
unit/race/integration tests are green. Go 1.26.

### Resume point (do this next — Phase 7)
1. ~~Embed the user docs + `karya help`/`karya docs`.~~ **Done** (#17).
2. ~~`karya tutorial` (self-working) + `Ctrl-a ?` in-session help.~~ **Done** (this branch).
3. ~~`karya doctor` — tools/versions/isolation + per-language tooling.~~ **Done** (this branch).
4. Distribution: `karya completion`, a Homebrew tap, and cut `v1.0.0`.
5. Provenance cleanup (final pass): strip every `nvim-config`/`dotfiles` reference
   from the whole repo and sever the build-time `../nvim-config` dependency.

### Phase 7 — what shipped so far
- **Embedded user docs (#17):** `internal/assets/docs.go` `go:embed`s `docs/*.md`
  with `DocTopics()`/`Doc(topic)` accessors. `scripts/sync-docs.sh` (+ `make
  sync-docs`) vendors `docs/*.md` → `internal/assets/docs/` (mirrors the
  `sync-nvim` idiom); a drift test (`internal/assets`) fails CI if the vendored
  copy falls out of sync, keeping `docs/*.md` the single source of truth.
- **CLI docs/help (#17):** `karya docs [topic]` pages the embedded docs offline
  (`$PAGER` → `less -R` → `more`, plain write for pipes/redirects); no topic lists
  them. `karya help [command]` prints rich per-command help (synopsis + syntax +
  guidance, pointing at `karya docs`); `karya help topics` lists every command.
  `-h`/`--help` still print the top-level usage.
- **`internal/tutorial` + `karya tutorial`:** a self-working tutorial — pure,
  side-effect-scoped lessons that execute real karya behavior against a throwaway
  `Sandbox` temp dir and verify it (isolation paths, `project.Scaffold`,
  `GitInit`, embedded docs, tmux availability, agent detection). Rendering writes
  to an injected `io.Writer`; the CLI (`karya tutorial [list|<n>]`) selects
  lessons and pauses between them only when interactive. Explanatory lessons carry
  a nil `Run`. Numbering is derived from slice order so it can't drift.
- **`Ctrl-a ?`:** tmux binding pops up the key map/command reference via
  `display-popup -E "<karya> docs keymaps"` (tmux ≥ 3.2).
- **`internal/doctor` + `karya doctor`:** health checks driven by an injected
  `Probe` (so the report logic is hermetically unit-tested; the CLI supplies a
  system-backed Probe via `exec.LookPath`, `tools.Installer.Available`,
  `agent.Detect`, …). Checks: karya version, isolation invariant (every dir
  namespaced under the karya prefix), embedded config extraction (nvim + tmux),
  core tools with versions (tmux/nvim essential → Problem when missing;
  git/mise/lazygit → Warn), detected coding agents, the language selection, and
  per-language + always-on editor tooling via `tools.Plan`. Three severity
  levels (`OK ✓ / Warn • / Problem ✗`); the command exits non-zero only on a
  Problem. Exposed `tools.Installer.Available` as the read-only tooling probe.

### Phase 6 — what shipped
- **`internal/update`:** self-update core with all network/OS side effects behind
  seams so the logic is hermetically tested (httptest, no real GitHub/network).
  `Updater.Latest` queries the GitHub Releases API; `FetchBinary` downloads the
  platform `karya_<ver>_<os>_<arch>.tar.gz` + `checksums.txt`, verifies the
  SHA-256, and extracts the binary (base-name match, traversal-guarded);
  `Apply` writes a temp file in the destination dir and renames over the running
  binary (atomic, same-filesystem). `IsNewer` does numeric (not lexical) semver
  compare and treats `dev` as always-updatable.
- **CLI:** `karya install` (isolated, non-destructive: extract configs → apply the
  current language selection → install runtimes + tools → sync editor plugins →
  print the `shellenv` hint); `karya update [--check]` (query → compare → fetch →
  verify → atomic replace → re-exec the **new** binary's `install` so the freshly
  shipped embedded configs/tools/plugins refresh); `karya uninstall` (removes only
  Config/Data/State/Cache + the binary, confirmed unless `-y`); `karya shellenv`.
- **`config.Paths.ShellEnv`:** opt-in `eval "$(karya shellenv)"` integration —
  puts the karya bin dir on PATH (guarded against duplicate evals), sets
  `$EDITOR/$VISUAL/$GIT_EDITOR`, and adds a `k` alias. Deliberately does **not**
  leak the managed tool bin / mise shims onto the global PATH — that toolchain is
  session-scoped via `config.Paths.Env`, preserving isolation.
- **`scripts/install.sh`:** POSIX `curl | sh` installer — detects OS/arch, resolves
  the latest tag (or `KARYA_VERSION`), downloads + checksum-verifies the archive,
  installs to `~/.local/bin/karya`, and runs `karya install`. Touches no rc file.
- **Tests:** version compare/`IsNewer`, asset naming, checksum parse/verify,
  tar.gz extraction (nested dir, missing binary, traversal reject), atomic
  `Apply`, full `Latest`+`FetchBinary` flow + checksum-mismatch/unsupported-
  platform via httptest; `ShellEnv` isolation; `confirm` prompt parsing. Gate
  green: `gofmt`, `go vet`, `golangci-lint` v2 (0 issues), `go test -race`,
  `-tags=integration`, `go build`.

### Phase 5 — what shipped
- **`internal/lang`:** pure, deterministic core — a language catalog (mise tool,
  fallback, dedup mode) with alias resolution; version discovery composed from a
  `VersionLister` interface (`MiseLister` shells out to `mise ls-remote`) →
  prerelease filtering → dedup by major/major.minor, with Java specially ranked
  per major (plain > temurin > corretto > zulu). Everything degrades to the
  catalog fallback offline. `Selection` is an ordered `languages.local` store
  (parse/render/round-trip); `GenerateMiseConfig` emits an **isolated** mise
  `config.toml` (`[tools]`, `JAVA_HOME`/`GOPATH`/`CARGO_HOME` under the karya
  prefix, `experimental`). `InstallRuntimes` is the thin `mise install` side
  effect.
- **`internal/tools`:** `Plan(langs)` (pure, tested) = always-on servers
  (lua_ls/json/yaml/bash/taplo/marksman) + per-language tools in catalog order.
  `Installer` is detect-first and best-effort: uv (→ `UV_TOOL_BIN_DIR`), npm
  (isolated `--prefix`), go (`GOBIN`), rustup components, and download-based
  jdtls/lombok/Java-DAP VSIX (tar/zip extraction with a traversal guard).
  Homebrew-class servers (lua_ls, clangd, taplo, marksman) are **detect-only**
  with an install hint — karya never runs `brew install`, honouring the
  no-Homebrew-mutation guarantee.
- **Isolation:** `config.Paths` gained `LanguagesFile`, `MiseConfig/Data/Cache`,
  `MiseEnv`, and `ToolsDir`; `Paths.Env` now pins mise inside the karya prefix
  and prepends the karya tool bin + mise shims to `PATH` for sessions — without
  touching the user's PATH, Homebrew, or global mise. Verified live: `gopls`,
  `dlv`, and the npm servers install under `~/.local/share/karya/tools/bin`.
- **CLI:** `karya lang [list|add <lang> [versions]|remove <lang>|all]` plus a
  bare-`karya lang` interactive selector; every mutation rewrites
  `languages.local`, regenerates the isolated mise config, installs runtimes, and
  installs tools, printing an install summary.
- **Tests:** dedup/Java-ranking/offline-fallback, selection round-trip, mise-gen,
  plan ordering, detect-first availability, archive traversal guard. Gate green:
  `gofmt`, `go vet`, `golangci-lint` v2 (0 issues), `go test -race`,
  `-tags=integration`, `go build`.

### Phase 4 — what shipped
- **`internal/project`:** pure, deterministic scaffolds for python/java/
  typescript/go/cpp/rust from embedded `text/template`s — a faithful port of
  `dotfiles/scripts/project-init.sh` (same files/layout/greetings). It never
  shells out to uv/cargo/go/npm, so generation is reproducible, offline, and
  hermetic to test. `NewSpec` normalises language aliases and derives the
  basename (last `/`- or `.`-separated component; module/group id preserved in
  `Name`); `className` yields a valid Java identifier. `Scaffold` refuses to
  overwrite an existing dir. `GitInit` is a separate best-effort side effect.
- **CLI `karya new`:** accepts `<lang> <name> [dir]` **and** the `lang:name`
  token form used by `Ctrl-a P`. Parent dir resolves as explicit arg → current
  session's `@ide_workdir` → cwd. After scaffolding + git init it opens the new
  project in its own IDE session (`session.Build` + `switch-client`) when run
  inside a karya session, else prints the `karya dev` launch hint.
- **Tests:** table-driven per-language scaffold assertions + basename/alias/
  class-name/dir-exists cases under `t.TempDir()`; `parseNewArgs` unit test for
  both invocation forms. Gate green: `gofmt`, `go vet`, `golangci-lint` v2,
  `go test -race`, `go test -tags=integration`, `go build`.

### Phase 3 — what shipped
- **Vendoring:** `scripts/sync-nvim.sh` (+ `make sync-nvim`) copies the runtime
  subset of `../nvim-config` (init.lua, lua/**, lazy-lock.json, after/) into
  `internal/assets/nvim/`, committed so `go:embed` works in CI/releases.
- **`internal/assets` (nvim):** `//go:embed all:nvim`; `NvimVersion` (sha256 over
  the sorted embedded tree), `ExtractNvimConfig` (clean re-extract + `manifest.json`),
  `EnsureNvimConfig` (extract only when missing/stale; reports whether it did).
- **Isolation primitive fixed:** `NVIM_APPNAME=karya/nvim` (was bare `karya`) so
  Neovim actually reads the extracted `~/.config/karya/nvim` and nests its
  data/state/cache under `…/karya/nvim`. `config.NvimAppName` centralises it;
  the session editor pane, `editor.execNvim` fallback, and session env all use it.
- **Startup wiring:** `newApp` calls `EnsureNvimConfig` every run (cheap hash +
  manifest compare); plugins bootstrap lazily via lazy.nvim on first editor launch.
  `editor.SyncPlugins` provides the headless `Lazy! sync` for `install`/`update`.
- **Isolation test:** an integration test drives real `nvim` under sandboxed HOME +
  XDG dirs and asserts every `stdpath` resolves under the karya prefix while the
  user's `~/.config/nvim` is never created. CI's integration job now installs nvim.

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
| 3 | Editor integration (embedded nvim) | ☑ done |
| 4 | Project scaffolding (`new`) | ☑ done |
| 5 | Language & tool management | ☑ done |
| 6 | Install / update / uninstall | ☑ done |
| 7 | Embedded help, self-guided tutorial, doctor & distribution | ◐ next |
| 8 | (Deferred) Native agent | ☐ |

---

## Changelog

### 2026-07-30 — Phase 6 complete: install / update / uninstall & self-update
- **`internal/update`** self-updates karya from GitHub Releases: query latest →
  download the platform `tar.gz` + `checksums.txt` → verify SHA-256 → extract the
  binary → atomically replace the running binary (temp file + rename). All side
  effects sit behind seams; the pure logic (numeric semver compare, asset naming,
  checksum parse/verify, tar.gz extraction with a traversal guard, atomic `Apply`)
  and the full network flow are unit-tested via `httptest` — no real network.
- **CLI lifecycle:** `karya install` (isolated, non-destructive setup), `karya
  update [--check]` (verify + atomic replace, then re-exec the new binary's
  `install` to refresh the freshly shipped configs/tools/plugins), `karya
  uninstall` (removes only the karya prefix + binary, confirmed unless `-y`),
  `karya shellenv` (opt-in PATH/`$EDITOR`/alias — session toolchain stays
  session-scoped, so nothing leaks onto the global PATH).
- **`scripts/install.sh`** is the `curl | sh` installer: OS/arch detection, tag
  resolution, checksum-verified download to `~/.local/bin/karya`, then `karya
  install`. No rc file, Homebrew, or global mise is touched.
- Gate green: `gofmt`, `go vet`, `golangci-lint` v2 (0 issues), `go test -race`,
  `go test -tags=integration`, `go build`.

### 2026-07-30 — Phase 5 complete: language & tool management (`karya lang`)
- **`internal/lang`** manages the language/version selection and generates an
  **isolated** mise config. Pure core (dedup by major/major.minor, Java
  distribution ranking, `languages.local` parse/render, mise-config gen) is
  unit-tested behind a `VersionLister` interface; `MiseLister`/`InstallRuntimes`
  are the thin `mise` side effects. Falls back to catalog defaults offline.
- **`internal/tools`** plans (always-on + per-language, pure/tested) and installs
  LSPs/formatters/adapters detect-first into the karya tool prefix: uv/npm/go/
  rustup + download-based jdtls/lombok/Java-DAP VSIX (with an archive-traversal
  guard). Homebrew-class servers are detect-only with a hint — no `brew install`.
- **Isolation:** `config.Paths` gained mise paths + `MiseEnv` + `ToolsDir`;
  `Paths.Env` pins mise inside the karya prefix and prepends the karya tool bin +
  mise shims to `PATH`, never touching the user's PATH, Homebrew, or global mise.
- **CLI:** `karya lang [list|add|remove|all]` + interactive selector; each
  mutation persists the selection, regenerates the mise config, and installs
  runtimes + tools. Verified live: tools land under `~/.local/share/karya/tools`.
- Gate green: `gofmt`, `go vet`, `golangci-lint` v2 (0 issues), `go test -race`,
  `go test -tags=integration`, `go build`.

### 2026-07-30 — Fix: agent-reset editor pane used the wrong NVIM_APPNAME
- **Bug:** `agent.recreateDevWindow` (rebuild path when the `dev` window was
  lost) launched the editor with a hardcoded `NVIM_APPNAME=karya` instead of
  `config.NvimAppName` (`karya/nvim`), so a rebuilt editor pane read the karya
  prefix root rather than the extracted nvim config — the Phase 3 isolation fix
  was missed here. Now uses `config.NvimAppName`; added a hermetic unit test
  (`TestRecreateDevWindowNamespacesNvim`) that guards it, and aligned the agent
  integration test's session env to the same value.
- **Docs:** corrected stale `NVIM_APPNAME=karya` → `karya/nvim` in PLAN §6.1,
  ROADMAP Phase 1, AGENT.md, and this file's next-session notes (dated changelog
  entries left as historical record; the Phase 3 entry documents the correction).

### 2026-07-30 — Phase 4 complete: project scaffolding (`karya new`)
- **`internal/project`** ports `project-init.sh`: `ParseLanguage` (aliases),
  `NewSpec` (basename/module/group derivation), and `Scaffold` writing pure,
  deterministic per-language file sets from embedded templates for python, java,
  typescript, go, cpp, rust. No external tools invoked → reproducible & offline.
  `GitInit` is a separate best-effort step; existing target dirs are refused.
- **CLI:** `karya new <lang> <name> [dir]` (+ `lang:name` form for `Ctrl-a P`).
  Parent dir = arg → session `@ide_workdir` → cwd; opens the project in its own
  IDE session via `session.Build` + `switch-client` when inside a karya session.
- **Tests:** hermetic table-driven scaffold tests under `t.TempDir()` +
  `parseNewArgs` cases. Gate green (`gofmt`/`vet`/`golangci-lint` v2/`-race`/
  `-tags=integration`/`build`). The `Ctrl-a P` keybinding was already wired.

### 2026-07-30 — Phase 3 complete: embedded, isolated Neovim editor
- **Vendoring:** `scripts/sync-nvim.sh` + `make sync-nvim` copy the runtime subset
  of `../nvim-config` into `internal/assets/nvim/` (committed — `go:embed` needs it
  in every checkout for CI and GoReleaser; removed the stale `.gitignore` exclude).
- **`internal/assets` (nvim):** `//go:embed all:nvim`; `NvimVersion` (deterministic
  sha256 over the sorted embedded tree), `ExtractNvimConfig` (clean re-extract that
  drops upstream-removed files + writes `manifest.json`), `EnsureNvimConfig` (extract
  only when missing/stale, returning whether it did). Hermetic `t.TempDir()` tests.
- **Isolation primitive corrected:** `NVIM_APPNAME=karya/nvim` (was bare `karya`,
  which pointed Neovim at an empty config). The `/nvim` suffix makes Neovim read the
  extracted `~/.config/karya/nvim` and nest data/state/cache under `…/karya/nvim`,
  separate from karya's own tmux.conf/prefs. Centralised as `config.NvimAppName` and
  used by the session editor pane, the `editor.execNvim` fallback, and the session env.
- **Startup wiring:** `newApp` runs `EnsureNvimConfig` each invocation (cheap hash +
  manifest compare, like tmux.conf). Plugins bootstrap lazily via lazy.nvim on first
  editor launch; `editor.SyncPlugins` exposes the headless `Lazy! sync` for Phase 6.
- **Isolation guarantee tested:** integration test launches real `nvim` under a
  sandboxed HOME + XDG env and asserts every `stdpath` resolves under the karya prefix
  while `~/.config/nvim` is never created. CI's integration job now installs neovim.
- Gate green: `gofmt`, `go vet`, `golangci-lint` v2, `go test -race`,
  `go test -tags=integration`, `go build`.

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
- Wrote [PLAN.md](PLAN.md) (full architecture + isolation model),
  [ROADMAP.md](ROADMAP.md), this file, and [AGENT.md](AGENT.md).
- Scaffolded repo: `go.mod`, `main.go`, `internal/{cli,config,version}`,
  `Makefile`, `.gitignore`, `assets/`. Every PLAN §4 command exists as a stub.
- **Blocker:** Go toolchain not installed — cannot `go build`/verify yet.

---

## Notes for the next session
- The **isolation model** (PLAN §2) is the defining constraint: never touch the
  user's `~/.zshrc`, `~/.tmux.conf`, `~/.config/nvim`, Homebrew, or global mise.
  Everything lives under the karya prefix; `NVIM_APPNAME=karya/nvim` +
  `tmux -L karya` are the isolation primitives.
- Source-of-truth behavior to port lives in:
  `dotfiles/scripts/dev.sh`, `ide-agent.sh`, `ide-run.sh`, `project-init.sh`,
  `languages.sh`; `dotfiles/{install,update,rebuild,link,doctor}.sh`;
  `dotfiles/bin/nvim-edit`; and the whole `nvim-config/` tree.
- Keep the dependency list minimal (stdlib → add `cobra` only when the tree
  grows). No CGO.
- **Keep ROADMAP.md and PLAN.md in sync** — when one changes, reflect it in
  the other (and this file).
- **Consolidating ≠ copying.** Port behavior from `nvim-config`/`dotfiles`
  deliberately; do not carry over their bugs or bad design decisions
  (e.g. dotfiles' symlink-over-user-config model — rejected in PLAN §2).
- **Phase 7 cleanup:** strip all `nvim-config`/`dotfiles` references from the
  **entire repo** — shipped surfaces (`--help`, README, docs, code comments)
  *and* the internal design log (PLAN, this file, AGENT) — and sever the
  build-time `../nvim-config` dependency. History stays in git. Tracked in
  ROADMAP Phase 7.
