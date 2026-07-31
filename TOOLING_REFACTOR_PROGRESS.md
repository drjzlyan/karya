# Tooling Refactor — Progress

Plan: see [TOOLING_REFACTOR.md](TOOLING_REFACTOR.md).
Branch: `feat/mise-always-on-and-install-onboarding`.

Status legend: ☐ not started · ◐ in progress · ☑ done

## Phase 1 — Registry core (pure, no behavior change) ✅
- ☑ `internal/toolreg` package: `Tool`, `Category`, `InstallMethod`, `UpdateStrategy`, `InstallLocation`, `HealthCheck`
- ☑ `catalog.go`: folded `tools/catalog.go` (alwaysOn + perLanguage) + `bootstrap.go` (core + toolchains) + `lang.Catalog` runtimes into one `registry []Tool` (31 tools)
- ☑ `registry.go`: `Registry` with `Get`/`All`/`ByCategory` + `Plan(ids)` dependency topo-sort
- ☑ Hermetic tests: Get/ByCategory, Plan topo-order/dedup/cycle/unknown, catalog-prefers-mise, well-formed
- ☑ `go test ./...` green; `golangci-lint` 0 issues on toolreg

## Phase 2 — Installer abstraction ✅
- ☑ `Method` interface + `base` + `Dispatcher` (`installer.go`); methods extracted to `methods.go`
      (mise/uv/npm/go/rustup/detect) and `download.go` (jdtls/lombok/vsix)
- ☑ `Installer` kept as thin facade over `Dispatcher` via `specToTool`/`kindToMethod` bridge —
      callers (doctor, cli) untouched; existing + new dispatcher tests green; lint 0 issues

## Phase 3 — Resolver + call-site threading ✅ (core; deeper threading deferred)
- ☑ `toolreg.Resolver.Resolve(id)` (managed→system→missing) + `Path` + tests over a temp prefix
- ☑ `toolreg.Manifest`/`WriteManifest`; `Paths.NvimData()`/`ToolsManifest()`; resolver+manifest on `app`,
      written in `newApp` (best-effort)
- ☑ `util/karya.lua`: `data_dir`/`cache_dir`/`manifest`/`tool` helpers (reads `<data>/karya-tools.json`)
- ☑ `util/java.lua` rewritten: resolves JDK/jdtls/lombok from karya's isolated prefix + manifest;
      **removed** the Homebrew/system-JVM/global-`~/.local/share/nvim` + `~/.cache/jdtls` leaks
- ◐ Deferred: routing doctor version probes + editor/session nvim/tmux launches through the resolver
      (they already resolve correctly via the isolated PATH; threading is additive, do with Phase 7)
- ☐ TODO: `util/health.lua` still has Homebrew fallbacks — clean up with Phase 7 doctor work

## Phase 4 — Expanded catalog + directory layout ✅ (data + infra; install rewire is Phase 5)
- ☑ Added core CLI tools (jq, yq, fd, ripgrep, fzf, bat, eza, delta, tree, zoxide, just, watchexec,
      hyperfine, shellcheck, shfmt, gh; git=detect) + doc tools (markdownlint, yamllint, yamlfmt, jsonlint)
- ☑ `config.Paths`: `ToolCategoryDir`/`ToolCategoryBin`/`ToolBinDirs`/`DownloadsDir`/`ToolsLogsDir`;
      `Env`/`ActivateManagedEnv` now prepend all category bins (ToolsBin first, then categories, then shims)
- ☑ Resolver searches all category bins (`ToolBinDirs`)
- ☑ Layout-aware `NewLayoutDispatcher(paths)` routes each tool into `tools/<category>` by `Location`;
      legacy `NewDispatcher` unchanged (shared prefix)
- ☑ `tools.Migrate` (idempotent, best-effort, no binary moves) wired into `newApp`; marker in state dir
- ☑ Tests: layout routing, migration idempotency, new-tool catalog presence; lint 0 issues
- Note: the new tools are provisioned once the install path is registry-driven (Phase 5).

## Phase 5 — ProfileManager + install rewire + CLI ✅ (RuntimeManager extraction deferred)
- ☑ `toolreg.Profile` + `Registry.Profiles/Profile/LanguageIDs/AlwaysOnIDs` (core & docs explicit;
      language profiles derived from `LocLang`); tests
- ☑ Install path is registry-driven: `applySelection` installs always-on + per-language via
      `installToolIDs` → `NewLayoutDispatcher` (category dirs); manifest refreshed after install
- ☑ `karya profile list|install <id>` (`cmdProfile`); dispatcher wired in `cli.go`
- ☑ `cmdInstall` installs the `core` + `docs` baseline profiles (best-effort)
- ☑ Dispatcher creates category bin/tools dirs on demand before install
- ☑ Smoke-tested `karya profile list`; build/tests/lint green
- Note: formal `RuntimeManager` type not extracted — runtime provisioning still inline in
  `applySelection` (`WriteMiseConfig` + `InstallRuntimes`); works, extraction is cosmetic, deferred.
- Note: `EnsureCore`/`EnsureToolchains` (bootstrap.go) still used by launch + `cmdInstall`; folding
  them into a `core` profile is deferred to the Phase 8 cleanup to avoid destabilizing launch.

## Phase 6 — Per-project isolation ✅
- ☑ `toolreg.DetectProject(dir)` walks up for mise.toml/.mise.toml/.tool-versions → `ProjectEnv`
- ☑ `config.Paths.EnvForProject(bin, root)` adds `MISE_TRUSTED_CONFIG_PATHS` (no global trust touched)
- ☑ Resolver `WithProject(pe)` + `SourceProject`: runtime tools resolved via `mise which` in the
      project dir (injectable for tests); falls back to managed
- ☑ `cmdDev` detects the project from the workdir and launches the session with `EnvForProject`, so
      shims layer the project's versions over global inside the session
- ☑ Tests: nearest-config detection, EnvForProject trust var, resolver prefers project + falls back
- Note: auto-provisioning a project's pinned runtime (`mise install` in the workdir on open) is not
  yet wired — project versions are used when already installed. Follow-up.

## Phase 7 — Health / update surface ✅ (via `karya tool`; deeper doctor+version deferred)
- ☑ `toolreg.HealthChecker` (`HealthStatus`: installed/version/location/source/repair-hint), injectable
      version probe; tests
- ☑ `Dispatcher.Reinstall` (force) primitive for updates
- ☑ `karya tool list` (health of all tools grouped by category) + `karya tool update <id>|all`
      (independent per-tool updates; pinned/manual reported, not touched)
- ☑ `util/health.lua`: removed Homebrew/system-JVM + global-`nvim/languages.local` leaks
      (now reads karya's isolated prefix + delegates lombok to `util.java`); brew hints → karya hints
- ◐ Deferred: `VersionManager` latest-vs-installed / update-available detection (LatestVersion stubs),
      `tools.state` LastValidated persistence, rollback, and folding health into `karya doctor`
      (the `karya tool` command is the primary health surface for now).

## Phase 8 — Cleanup
- ☐ Delete retired shims/dead code; update docs; `//go:build integration` end-to-end

## Notes / decisions
- Registry = Go struct literals (zero-dep rule); no TOML parser.
- `data/mise` never relocated (moving mise installs breaks shims).
- Isolation leak found: `internal/assets/nvim/lua/util/java.lua` reads the user's global
  `~/.local/share/nvim/languages.local` and `~/.local/share/mise/...` — fixed in Phase 3.
