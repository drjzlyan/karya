# karya Tool-Management Refactor — End-to-End Plan

## Context

karya is an AI-first IDE (pure Go, stdlib-only core) that orchestrates Neovim + tmux and
runs every tool from an **isolated, vendored `mise`** under a karya-owned XDG prefix. It
already has the seeds of a good tool system: a vendored mise (`internal/tools/mise.go`), a
`ToolSpec` catalog with install "Kinds" (`internal/tools/catalog.go`, `install.go`), a
detect-first installer, PATH-based managed-tool resolution (`config.Paths.ActivateManagedEnv`),
per-language install "plans", isolated mise-config generation (`internal/lang/mise.go`), and a
`doctor` health surface.

**Why change it.** The current design has real gaps that block the product goals of
reproducible, fully-isolated, per-project environments:

- **No explicit resolver.** Tool resolution relies on PATH ordering; there is no single
  `Resolve(id)` seam, so call sites (nvim lua, doctor, editor/session launch) each re-derive
  paths — and some drift into hardcoded, non-isolated paths.
- **Isolation leak + bug.** `internal/assets/nvim/lua/util/java.lua` hardcodes
  Homebrew/system JDK + lombok paths, and worse, reads the **user's** global
  `~/.local/share/nvim/languages.local` and `~/.local/share/mise/installs/java/` instead of
  karya's isolated `~/.local/share/karya/...` prefix (lines 9, 55–57, 137–160). This violates
  the ironclad isolation rule and silently misresolves JDKs.
- **Thin metadata.** Tools have no category, health check, version, dependency, or update
  strategy — so no health/version/update surface is possible.
- **Few managed tools.** Core CLI (jq, ripgrep, fd, fzf, …) and doc tools (prettier,
  markdownlint, yamllint, …) are assumed global, contradicting "zero dependency on global
  tools."
- **No per-project isolation.** Everything is pinned to global managed versions; a project's
  own `mise.toml`/`.tool-versions` is ignored.

**Outcome.** A full manager architecture (ToolRegistry, Installer, Resolver, ProfileManager,
VersionManager, HealthChecker, RuntimeManager, UpdateManager) where every tool invocation
goes through one resolver, more tool categories are managed, health/version/update are
surfaced in `doctor`, and project-specific versions override global ones — **without ever
breaking isolation**. Delivered as one plan across sequenced, independently-shippable phases.

**Guiding decisions**
- **Registry as Go struct literals**, not embedded TOML. Zero-dep rule (AGENT.md) is ironclad
  and stdlib has no TOML parser; a hand-rolled parser is more risk than value. Catalog stays a
  pure, table-tested Go data table.
- **Evolve, don't fork.** `tools.ToolSpec`→`toolreg.Tool`; `tools.Kind`→`InstallMethod`;
  `tools.Installer` switch → `Method` interface + `Dispatcher`; `lang` runtime logic →
  `RuntimeManager`. Retired exports get thin shims during migration, then are deleted.
- **Isolation invariant unchanged.** Everything resolves under `config.Paths`; the Resolver
  reads only karya-owned locations; `data/mise` is **never** relocated (moving mise installs
  breaks shims).

---

## Target package layout

New pure core package `internal/toolreg`; `internal/tools` becomes the side-effect install
layer; `internal/lang` grows a `RuntimeManager`; CLI grows `profile`/`tool` subcommands.

```
internal/toolreg/            NEW — pure data + resolution core
  tool.go        Tool, Category, InstallMethod, UpdateStrategy, HealthCheck, InstallLocation
  catalog.go     registry []Tool  (folds tools/catalog.go + bootstrap.go + lang.Catalog runtimes)
  registry.go    Registry: Get/ByCategory/All + Plan(ids) with dependency topo-sort
  profile.go     Profile + profiles []Profile + ProfileManager
  resolver.go    Resolver.Resolve(id) -> Resolved{Path,Version,Source}
  health.go      HealthChecker.Check(tool) -> HealthStatus
  version.go     VersionInfo + VersionManager (reuse update.compareVersions)
  project.go     DetectProject(dir) -> *ProjectEnv (pure fs walk + parse)
  migrate.go     idempotent legacy-layout migration
internal/tools/              EVOLVED — side-effect install layer only
  installer.go   Method interface + Dispatcher (was install.go switch)
  method_*.go    mise/uv/npm/go/rustup/binary/jdtls/lombok/vsix/detect (bodies lifted verbatim)
  mise.go, download.go       shared low-level helpers (largely unchanged)
internal/lang/               EVOLVED — RuntimeManager (from runtimes.go + mise.go generate)
internal/doctor/doctor.go    EVOLVED — Probe gains Registry + Health + Versions
internal/config/paths.go     EVOLVED — category dirs + EnvForProject layering
internal/cli/                EVOLVED — profile.go, tool.go; install/lang/doctor rewired
internal/assets/nvim/lua/    EVOLVED — java.lua leak removed; tools.json manifest + util/tools.lua
```

Retired at end: `internal/tools/catalog.go`, `internal/tools/bootstrap.go`, the monolithic
`install.go` switch.

---

## The eight components (key types)

**1. ToolRegistry** — `internal/toolreg/tool.go`
```go
type Category string       // Runtime, LanguageServer, Formatter, Linter, Debugger, BuildTool, CLIUtility
type InstallMethod string  // Mise, UV, NPM, Go, Rustup, Binary, JDTLS, Lombok, VSIX, Detect
type UpdateStrategy string // Latest, Pinned, Mise, Manual
type InstallLocation ...   // LocCore, LocDocs, LocLang(name), LocRuntime  → selects on-disk dir

type Tool struct {
    ID, Name     string
    Category     Category
    Method       InstallMethod
    Executable   string   // bin name ("" for artifact-only, e.g. lombok)
    Artifact     string   // ToolsDir-relative artifact (lombok.jar, jdtls)
    Pkg          string   // method-specific: registry key / module / URL template
    Version      string   // "" == latest
    Dependencies []string // tool IDs installed first (jdtls -> java-runtime)
    Update       UpdateStrategy
    Location     InstallLocation
    Hint         string   // for MethodDetect
    Health       HealthCheck // {VersionArgs []string; Probe string}
}
```
`Registry.Plan(ids) ([]Tool, error)` replaces `tools.Plan`/`PlanFor` with a deterministic
dependency topo-sort. Every current `alwaysOn`/`perLanguage`/`coreTools`/`toolchainTools`
entry and each `lang.Catalog` runtime row becomes a `Tool`.

**2. Installer abstraction** — `internal/tools/installer.go`
```go
type Method interface {
    Available(t toolreg.Tool, ctx Context) bool
    Install(t toolreg.Tool, ctx Context) error
    CurrentVersion(t toolreg.Tool, ctx Context) string
    LatestVersion(t toolreg.Tool, ctx Context) string
}
type Dispatcher struct{ methods map[toolreg.InstallMethod]Method }
```
Each `method_*.go` wraps an existing `installUV/installNPM/installGo/installRustup/installMise/
installJDTLS/installLombok/installVSIX` body verbatim — near-mechanical extraction. `Result`
(`{Tool,Status,Err}`) and `Summarize` are kept. Install-order preference
`mise → language-native → binary → jdtls/lombok/vsix → detect` is a catalog-authoring
guideline enforced by a `TestCatalogPrefersMise` validation test (maximizes isolation via the
single vendored provisioner).

**3. Resolver** — `internal/toolreg/resolver.go` — the single execution seam
```go
type Source int // SourceProject > SourceManaged > SourceSystem > SourceMissing
type Resolved struct{ ID, Path, Version string; Source Source }
func (rv *Resolver) Resolve(id string) (Resolved, bool)
func (rv *Resolver) Path(id string) string
```
Resolution order: project mise (`mise which` run with project env, Phase 6) → category bin →
legacy `ToolsBin()`/`MiseShims()` → `exec.LookPath` under managed env → missing. Lives on the
`app` struct (`internal/cli/app.go`), built in `newApp` after `ActivateManagedEnv`, shared by
every command. **nvim lua:** hybrid — keep PATH-prepend as fallback, but write a `tools.json`
manifest into `NvimConfig()` from the Resolver; a new `util/tools.lua` reads it with
`vim.fn.exepath` fallback. This lets us **delete the java.lua hardcoded/global paths** and read
`jdtls`, the JDK (→ `JAVA_HOME`), and `lombok.jar` from the manifest instead.

**4. ProfileManager** — `internal/toolreg/profile.go`
```go
type Profile struct{ ID, Name string; Tools []string; Runtimes []string }
```
Profiles: `core`, `docs`, `python`, `node`, `rust`, `go`, `java`, `cpp`. Installing =
`Registry.Plan(profile.Tools)` → `Dispatcher.Install` + `RuntimeManager.Ensure(runtimes)`.
Subsumes `EnsureCore` (→ core profile) and `EnsureToolchains` (→ toolchain tools).
CLI: `karya profile list|install <id>|status`.

**5. VersionManager** — `internal/toolreg/version.go`
```go
type VersionInfo struct{ ID, Installed, Latest string; UpdateAvailable, CanRollback bool }
```
Independent per-tool updates; rollback for versioned artifacts (jdtls/lombok) by re-pointing a
retained symlink. Reuses `update.compareVersions`.

**6. HealthChecker** — `internal/toolreg/health.go`
```go
type HealthStatus struct{ ID string; Installed, Executable bool; Version, Location string;
    Source Source; LastValidated time.Time; RepairHint string }
```
`LastValidated` persists in `state/karya/tools.state` (key=value, like `prefs`).

**7. RuntimeManager** — `internal/lang/runtime.go` — extracted from `runtimes.go` +
`mise.go` generate. `Selection` (`languages.local`) stays the runtime source of truth;
`languages.local` format is unchanged.

**8. UpdateManager** — independent updates: `karya tool update <id>`,
`karya update tools [--category X]`. Reuses existing `internal/update` self-update plumbing.

---

## New managed tools (capability 2)

All `Location: LocCore` (or `LocDocs`), mostly `MethodMise`:
- **Core CLI** (mise): gh, jq, yq, fd, ripgrep(rg), fzf, bat, eza, delta, tree, zoxide, just,
  watchexec, hyperfine, shellcheck, shfmt. **git** stays `MethodDetect` (system git is fine;
  mise-git is heavy) — hint if absent.
- **Doc tools**: prettier (npm, Formatter), markdownlint (npm `markdownlint-cli2`, Linter),
  taplo (mise, already present), yamllint (uv, Linter), yamlfmt (mise, Formatter), jsonlint
  (npm, Linter).

---

## Per-project isolation (capability 4)

`DetectProject(dir)` walks up for `mise.toml`/`.mise.toml`/`.tool-versions`/`.git` (pure).
**Layering exploits mise's native config hierarchy** rather than replacing the global pin:
- Run mise-backed resolution/exec with `cmd.Dir = projectRoot` so mise discovers the local
  config layered over karya's global config.
- Keep `MISE_DATA_DIR`/`MISE_CACHE_DIR`/`MISE_STATE_DIR` and `MISE_GLOBAL_CONFIG_FILE` pinned
  to the karya prefix (versions the project asks for install into karya's isolated prefix).
- Add `MISE_TRUSTED_CONFIG_PATHS=<projectRoot>` so the project file is trusted
  non-interactively **without touching the user's global mise trust store**.

New `func (p Paths) EnvForProject(karyaBin, projectRoot string) []string` (additive to today's
global-only `Env`/`ActivateManagedEnv`). Session/editor child env + the Resolver's `mise which`
run in the project context, so `gopls`/`python`/etc. resolve project-pinned runtimes first.

---

## On-disk layout + migration

Target under `paths.Data`: `tools/{core,docs,python,node,go,rust,java,cpp}` plus
`downloads/`, `profiles/`, `logs/`, `temp/`. `config.Paths` grows
`ToolsCategoryDir(loc)`, `CategoryBin(loc)`, `DownloadsDir()`, `ProfilesDir()`, `LogsDir()`,
`TempDir()`; each `Tool.Location` selects its dir. **`data/mise` is NOT moved** — `runtimes/`
is nominal only.

Migration (`internal/toolreg/migrate.go`, run once from `newApp`, idempotent, guarded by
`state/karya/layout.version`) is **best-effort**: move known binaries into category dirs; the
Resolver keeps legacy `tools/bin`, `data/mise/shims`, and `ToolsDir/lombok.jar` as permanent
fallback lookup dirs, so existing users work with zero action. `karya install`/`doctor`
migrate opportunistically.

---

## Phased delivery (each independently shippable + tested)

1. **Registry core** (pure, no behavior change). Add `internal/toolreg` Tool/Category/
   InstallMethod/Registry + catalog folded from `tools/catalog.go`, `bootstrap.go`,
   `lang.Catalog`. `tools.Plan` delegates to `Registry.Plan`. Table tests + `TestCatalogPrefersMise`.
2. **Installer abstraction.** Extract `install.go` switch into `Method` impls + `Dispatcher`;
   wire `applySelection`/`cmdInstall` via adapter. Behavior identical; existing `tools_test.go`
   stays green.
3. **Resolver + call-site threading.** Add `Resolver` on `app`; route doctor version probes and
   editor/session nvim/tmux lookups through it (managed+system only). Emit `tools.json` +
   `util/tools.lua`; **delete java.lua Homebrew/global hardcodes**. Resolver unit tests over a
   temp prefix; lua manifest render test.
4. **Expanded catalog + directory layout.** Add ~24 CLI/doc tools with `Location`, category
   dirs + `Paths` helpers, best-effort migration with fallback lookups. Layout + migration tests.
5. **ProfileManager + RuntimeManager + CLI.** Add `Profile`/`ProfileManager`, refactor
   `EnsureCore`/`EnsureToolchains` into `core` profile, extract `RuntimeManager`. Add
   `karya profile …`; rewire `cmdInstall` to install core + selected language profiles.
6. **Per-project isolation.** Add `DetectProject`, `Paths.EnvForProject`,
   `MISE_TRUSTED_CONFIG_PATHS`, `cmd.Dir=projectRoot` in session/resolver exec; Resolver gains
   project source. Pure walk tests; resolver-prefers-project test with fake `mise which`.
7. **Health/Version/Update surfaced in doctor.** Add `HealthChecker`, `VersionManager`,
   `UpdateManager`; `doctor` consumes Health + optional `--check-updates`; `tools.state`
   persists LastValidated; rollback for jdtls/lombok. Fake-Method tests + doctor rendering test.
8. **Cleanup.** Delete retired shims (`tools.Plan`/`PlanFor`, `ToolSpec`, `Kind`),
   `catalog.go`/`bootstrap.go`, dead lua branches. Update `docs/`/help. `//go:build integration`
   end-to-end: real profile install → resolve → doctor; project layering with a scratch repo.

Spine: **1 → 2 → 3** (registry, installer, resolver). 4–8 are largely additive; ProfileManager
(5) precedes the `install` rewrite, and per-project (6) precedes surfacing project versions (7).

---

## Files to modify (representative)

- `internal/tools/catalog.go`, `install.go`, `bootstrap.go` — fold into `toolreg` + `method_*.go`.
- `internal/config/paths.go` — category dirs, `EnvForProject`, `MISE_TRUSTED_CONFIG_PATHS`.
- `internal/doctor/doctor.go` — `Probe` gains Registry/Health/Versions; `checkLanguages`+
  `checkCoreTools` → registry-driven `checkTools`.
- `internal/lang/mise.go`, `runtimes.go` — extract `RuntimeManager`.
- `internal/cli/app.go` (Resolver on `app`), `install.go`, `lang.go`, plus new `profile.go`,
  `tool.go`.
- `internal/assets/nvim/lua/util/java.lua` — remove hardcoded/global paths, read `tools.json`;
  new `internal/assets/nvim/lua/util/tools.lua`.
- Internal docs per memory `roadmap-plan-in-sync`: update `PLAN.md` §6.4, `ROADMAP.md`,
  `PROGRESS.md` together as phases land.

---

## Verification

- **Per phase (hermetic, default):** `make test` — registry Plan/topo/dedup, catalog
  validation, profile resolution, `DetectProject` walk, migration over a fabricated legacy temp
  prefix, resolver over a temp prefix, doctor rendering with injected fake Health/Versions, lua
  manifest render. No network, no real mise.
- **Lint/format gate:** `make gate` (fmt-check, vet, golangci-lint — run via `go run` per memory
  `golangci-lint-via-go-run`, race + integration tests, build).
- **Integration (`//go:build integration`):** real mise install of one tool into a temp prefix;
  `karya profile install python` → Resolver returns managed `ruff`/`python` → `karya doctor`
  reports OK; per-project layering by dropping a `.tool-versions` in a scratch repo and asserting
  the Resolver prefers it.
- **Manual smoke:** on a clean prefix, `karya install` → `karya doctor` (all core OK, tools
  healthy) → `karya` (session launches; nvim resolves `gopls`/`jdtls`/`lombok.jar` via manifest;
  no Homebrew/global paths consulted) → open a project with `.tool-versions` and confirm the
  pinned runtime is used.

## Risks
- **Zero-dep vs registry format** — resolved by Go literals (no parser).
- **Moving mise data breaks shims** — plan keeps `data/mise` in place.
- **nvim manifest staleness** — mitigated by PATH-prepend fallback (hybrid).
- **mise trust prompts** — scoped `MISE_TRUSTED_CONFIG_PATHS`, never the user's global trust file.
- **Large Resolver call-site surface** — phased: Phase 3 managed/system only; project layer (6)
  additive.
