# karya — Roadmap

Phased build order. Each phase is shippable and leaves the binary working.
Track live status in [PROGRESS.md](PROGRESS.md). Full design in
[PLAN.md](PLAN.md).

Legend: ☐ not started · ◐ in progress · ☑ done

---

## Phase 0 — Scaffold & CLI skeleton
**Goal:** a buildable single binary with the command tree stubbed out.

- ☐ Go module, `main.go`, `internal/cli` dispatch (stdlib `flag` for now)
- ☐ `internal/config` — XDG-based karya paths + prefix resolution
- ☐ `internal/version` — version string, `karya version`
- ☐ Stub every command from PLAN §4 (print "not yet implemented")
- ☐ `Makefile` (build/test/fmt/vet), `.gitignore`
- ☐ CI (GitHub Actions: build + vet + test on macOS/Linux)
- ☐ `goreleaser` config for cross-compiled release artifacts

**Done when:** `go build` produces `karya`; `karya version` and `karya --help`
work; every documented command exists as a stub.

---

## Phase 1 — Session orchestration (`dev`) ✅
**Goal:** `karya` launches the isolated tmux IDE session.

- ☑ `internal/tmuxx` — tmux wrapper on dedicated socket `-L karya`
- ☑ Embed + extract `tmux.conf` (`internal/assets`); launch server with `-f`
- ☑ `internal/session.Dev` — editor/agent/build panes + git window + `@ide_*` state
- ☑ Session env: `NVIM_APPNAME=karya/nvim`, `EDITOR/VISUAL/GIT_EDITOR=karya edit`
- ☑ `karya edit <file> [line]` (port of `nvim-edit`)
- ☑ `karya run [-d dir] <cmd>` / `--focus` (port of `ide-run`)
- ☑ `-k` recreate / `-q` quit / attach-if-exists
- ☑ Verified: layout, isolated env, `@ide_*` state; default tmux server untouched

**Done when:** `karya` opens the 3-pane layout on a karya-only tmux socket with
the user's tmux/nvim untouched; `karya edit` and `karya run` route correctly.

---

## Phase 2 — Agent management (AI-first core) ✅
**Goal:** first-class agent detection, switching, and memory.

- ☑ `internal/agent` — detect `crush/claude/codex/gemini/aider/copilot`
- ☑ Resolution: flag → per-project pref → single → interactive picker
- ☑ `internal/prefs` — per-project `key=value` store under karya prefix
- ☑ `karya agent status|switch|next|prev|reset|prefs|clear`
- ☑ Wire tmux keybindings (`Ctrl-a A/N/D/P/S`) to karya commands
- ☑ Faithful port of `ide-agent.sh` respawn/reset semantics

**Done when:** agent selection, cycling, reset, and per-project memory match
today's behavior, driven entirely by `karya`.

---

## Phase 3 — Editor integration (embedded Neovim config) ✅
**Goal:** the Neovim IDE ships inside the binary, fully isolated.

- ☑ Vendor `nvim-config` into `internal/assets/nvim/` (`scripts/sync-nvim.sh` / `make sync-nvim`)
- ☑ `go:embed` + extract to `~/.config/karya/nvim`; version via `manifest.json`
- ☑ Launch via `NVIM_APPNAME=karya/nvim`; isolated data/state/cache dirs
- ☑ Plugin bootstrap/sync (`editor.SyncPlugins` → `nvim --headless +"Lazy! sync" +qa`)
- ☑ Verify user's `~/.config/nvim` is never read/written (isolation test)

**Done when:** editing works with the full LSP/DAP/git/tasks config, extracted
from the binary, with zero impact on the user's own Neovim.

---

## Phase 4 — Project scaffolding (`new`) ✅
**Goal:** `karya new <lang> <name>` for all six languages.

- ☑ `internal/project` scaffolds: python, java, typescript, go, cpp, rust
- ☑ `git init`; open in an IDE session when inside a karya session
- ☑ `Ctrl-a P` prompt (`lang:name`) → `karya new`

**Done when:** each language produces the same scaffold as `project-init.sh`.

---

## Phase 5 — Language & tool management (`lang`, tools) ✅
**Goal:** isolated language/version selection and tool install.

- ☑ `internal/lang` — selector + version discovery from `mise ls-remote`
  (dedup by major/minor, Java distribution ranking), offline fallback
- ☑ Write `languages.local`; generate **isolated** mise config
  (`MISE_GLOBAL_CONFIG_FILE`/`MISE_DATA_DIR` pinned to the karya prefix)
- ☑ `internal/tools` — detect-or-install LSPs/formatters/adapters into the karya
  tool prefix (uv/npm/go/rustup + jdtls/lombok/VSIX downloads); Homebrew-class
  servers are detect-only with a hint (never `brew install`)
- ☑ Always-on servers + per-language selectable servers (PLAN §6.4)
- ☑ `karya lang [list|add|remove|all]` + interactive selector

**Done when:** selecting a language installs its tooling into the karya prefix
without modifying Homebrew or the user's global mise.

---

## Phase 6 — Install / update / uninstall & self-update ✅
**Goal:** lifecycle management, including binary self-replacement.

- ☑ `karya install` — extract configs, fetch tools, no user-setting changes
- ☑ `karya update [--check]` — self-replace binary + re-extract configs + tools + `Lazy! sync`
- ☑ `karya uninstall` — remove karya prefix + binary only (confirmed, `-y` to skip)
- ☑ GitHub Releases integration + checksum verification + atomic replace
  (`internal/update`, hermetically tested via httptest)
- ☑ `curl | sh` install script (`scripts/install.sh`)
- ☑ `karya shellenv` (opt-in PATH/alias/EDITOR; session toolchain stays session-scoped)

**Done when:** a user can install via one command, update in place, and fully
uninstall leaving no trace beyond their own pre-existing config.

---

## Phase 7 — Embedded help, self-guided tutorial, doctor & distribution
**Goal:** production-ready release where the docs *ship inside the binary* — a
user who has only `karya` on their PATH can learn and use it fully offline,
without the repo or a browser.

### Embedded help & tutorial (in the binary, not just `docs/`)
- ☑ `internal/assets` — `go:embed` the user docs (`docs/tutorial.md`,
  `docs/keymaps.md`) so they travel with the binary (`make sync-docs` vendors them)
- ☑ `karya help [command]` — rich, per-command help beyond the flag usage `flag`
  prints, pointing at the embedded docs; `karya help topics` lists the commands
- ☑ `karya tutorial` — a **self-guided, self-working** tutorial launched from the
  binary: numbered lessons a user can step through (`karya tutorial [lesson]`),
  each runnable against a throwaway sandbox so commands actually execute and are
  verified (isolation, scaffold, git-init, embedded docs, tmux, agents)
- ☑ `karya docs [topic]` — browse the embedded docs offline (pager/`$PAGER`)
- ☑ Single source of truth: `docs/*.md` are the source, embedded at build time; a
  test asserts the embedded content stays in sync with `docs/`
- ☑ `Ctrl-a ?` keybinding opens the in-session help (key map & command reference)

### Doctor & distribution
- ☐ `karya doctor` — tools, versions, isolation checks, per-language tooling
- ☐ `docs/` complete (isolation, commands, languages, troubleshooting)
- ☐ Shell completions (`karya completion`)
- ☐ Homebrew tap
- ☐ Tag `v1.0.0`, CHANGELOG, release automation

### Provenance cleanup (final pass — karya stands alone)
- ☐ Remove **every** reference to the source projects (`nvim-config`, `dotfiles`)
  from the entire repo — karya is the product, not a port. This covers both
  shipped surfaces (`--help`, README, `docs/`, in-code comments) **and** the
  internal design log (`PLAN.md`, `PROGRESS.md`, `AGENT.md`): rewrite
  "port of X.sh" / consolidation-map language to describe karya on its own
  terms. Historical provenance lives in git history, not the tree.
- ☐ Sever the build-time dependency on `../nvim-config`: the vendored
  `internal/assets/nvim/` becomes the sole source of truth; `scripts/sync-nvim.sh`
  is either retired or repointed so a clean checkout builds with no sibling repos.

**Done when:** clean install → working AI IDE → `karya doctor` all-green on a
fresh macOS machine, and `karya help`/`karya tutorial` teach the whole workflow
**offline from the binary alone**, with no repo checkout or network needed —
and no shipped surface still refers to `nvim-config`/`dotfiles`.

---

## Phase 8 — (Deferred) Native agent option
**Goal:** optional built-in LLM agent behind the existing agent interface.

- ☐ Define pluggable agent interface (BYO-CLI vs native)
- ☐ Native agent loop (tool-use, edits, run) using the Claude API
- ☐ Config for keys/models; keep BYO-CLI as default

**Status:** deferred by decision; the Phase 2 interface must not preclude it.
