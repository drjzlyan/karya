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

## Phase 4 — Expanded catalog + directory layout
- ☐ Add ~24 CLI/doc tools with `Location`; category dirs + `Paths` helpers; best-effort migration

## Phase 5 — ProfileManager + RuntimeManager + CLI
- ☐ `Profile`/`ProfileManager`; refactor `EnsureCore`/`EnsureToolchains` into `core` profile
- ☐ Extract `RuntimeManager`; add `karya profile …`; rewire `cmdInstall`

## Phase 6 — Per-project isolation
- ☐ `DetectProject`, `Paths.EnvForProject`, `MISE_TRUSTED_CONFIG_PATHS`, `cmd.Dir=projectRoot`
- ☐ Resolver project source (highest priority)

## Phase 7 — Health/Version/Update in doctor
- ☐ `HealthChecker`, `VersionManager`, `UpdateManager`; doctor `--check-updates`; `tools.state`; rollback

## Phase 8 — Cleanup
- ☐ Delete retired shims/dead code; update docs; `//go:build integration` end-to-end

## Notes / decisions
- Registry = Go struct literals (zero-dep rule); no TOML parser.
- `data/mise` never relocated (moving mise installs breaks shims).
- Isolation leak found: `internal/assets/nvim/lua/util/java.lua` reads the user's global
  `~/.local/share/nvim/languages.local` and `~/.local/share/mise/...` — fixed in Phase 3.
