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

## Phase 8 — Cleanup ✅ (integration test deferred)
- ☑ Migrated the last legacy callers to the registry: `doctor` (Probe now has `Registry` +
      `ToolInstalled(id)`, driven by the resolver) and `alwaysOnMiseTools` (toolreg-based)
- ☑ Deleted `internal/tools/catalog.go` (ToolSpec/Kind/alwaysOn/perLanguage/Plan/PlanFor/AlwaysOnMise);
      trimmed `install.go` to `Status`/`Result`/`Summarize`; removed the `Installer` facade + bridge;
      pruned obsolete tests
- ☑ `karya profile`/`karya tool` added to usage, `karya help`, and shell completion
- ☑ Full suite green, `golangci-lint ./...` 0 issues; smoke-tested `karya tool list` + `karya help profile`
- ◐ Deferred: `//go:build integration` end-to-end (real mise profile install → resolve → doctor) — needs
      network + mise; unit coverage is comprehensive.

---

## Summary
All eight phases landed as sequenced, independently-green commits. The tool system now has a single
pure registry (`internal/toolreg`) with a metadata model, a pluggable installer `Dispatcher` with
category-based layout, a unified `Resolver` (managed→system, project-layered), profiles, per-project
isolation, and a health/update surface (`karya tool`, `karya profile`). The `java.lua`/`health.lua`
isolation leaks are fixed. Remaining deferred items (all noted above): formal `RuntimeManager`
extraction, folding `bootstrap.go` into a core profile, `VersionManager` latest/rollback +
`tools.state`, auto-provisioning project-pinned runtimes on open, and the integration test.

## Deferred follow-ups ✅ (branch `feat/tooling-deferred-followups`)
The six deferred items are now implemented:
- ☑ **RuntimeManager** extracted (`internal/lang/runtime.go`); `applySelection` uses it.
- ☑ **bootstrap.go retired** — launch (`ensureRuntime`) and install (`cmdInstall`) use the registry
      dispatcher via `app.ensureCore()` (+ `Registry.EssentialIDs`); toolchains now arrive as tool
      dependencies. `internal/tools/bootstrap.go` deleted.
- ☑ **Version/health surface** — `toolreg.VersionManager` (mise `outdated`-centric),
      `karya tool list --check-updates`, `karya doctor --check-updates`, `tools.state`
      (`Paths.ToolsStateFile`, via `prefs.Store`).
- ☑ **Per-project auto-provision** — `tools.ProvisionProject` run by `cmdDev` on open (best-effort),
      with a `-P`/`--no-provision` escape.
- ☑ **Integration test** — `internal/tools/e2e_integration_test.go` (`//go:build integration`)
      drives registry→dispatcher→resolver→health→version→doctor against a fake mise; CI-friendly.
- ☑ **Resolver threading** — decided: threaded into diagnostics only (doctor/health/version);
      tmux/nvim/session launches stay on the isolated PATH by design (documented).
- Still deferred (low value): rollback for pinned artifacts (jdtls/lombok).

## Notes / decisions
- Registry = Go struct literals (zero-dep rule); no TOML parser.
- `data/mise` never relocated (moving mise installs breaks shims).
- Isolation leak found: `internal/assets/nvim/lua/util/java.lua` reads the user's global
  `~/.local/share/nvim/languages.local` and `~/.local/share/mise/...` — fixed in Phase 3.
