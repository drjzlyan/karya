// Package toolreg is karya's pure tool-metadata core: a registry of every tool
// karya manages (runtimes, language servers, formatters, linters, debuggers,
// build tools, and CLI utilities), each described by category, install method,
// version, dependencies, install location, update strategy, and how to health
// check it.
//
// This package holds no side effects. Installation lives in internal/tools,
// runtime provisioning in internal/lang, and resolution/health/version/update
// layers are built on top of the registry. Keeping the data and the pure query
// logic here — deterministic and hermetically unit-tested — is what lets the
// rest of karya route every tool decision through one source of truth instead of
// scattering executable names and install logic across the codebase.
package toolreg

// Category groups tools by role so the registry can be queried by kind and so
// diagnostics (doctor) can present tools grouped meaningfully.
type Category string

const (
	// Runtime is a language runtime provisioned via mise (python, node, go, …).
	Runtime Category = "runtime"
	// LanguageServer is an LSP server (gopls, basedpyright, jdtls, …).
	LanguageServer Category = "lsp"
	// Formatter formats source (prettier, taplo, rustfmt, …).
	Formatter Category = "formatter"
	// Linter lints source (ruff, clippy, …).
	Linter Category = "linter"
	// Debugger is a debug adapter (debugpy, delve, java-debug, …).
	Debugger Category = "debugger"
	// BuildTool is a compiler/build/package helper (tsc, uv, …).
	BuildTool Category = "buildtool"
	// CLIUtility is a general command-line tool karya runs (tmux, nvim, jq, …).
	CLIUtility Category = "cli"
)

// InstallMethod is how a tool is provisioned into karya's isolated prefix. Each
// method maps to one installer in internal/tools. The preferred authoring order
// is mise → language-native (uv/npm/go/rustup) → binary → jdtls/lombok/vsix →
// detect: mise maximizes isolation via the single vendored provisioner.
type InstallMethod string

const (
	// MethodMise installs from mise's registry into karya's isolated prefix.
	MethodMise InstallMethod = "mise"
	// MethodUV installs a Python tool via `uv tool install`.
	MethodUV InstallMethod = "uv"
	// MethodNPM installs a Node package into karya's isolated npm prefix.
	MethodNPM InstallMethod = "npm"
	// MethodGo installs a Go tool via `go install` with GOBIN in karya's bin.
	MethodGo InstallMethod = "go"
	// MethodRustup adds a rustup component (the one class karya cannot fully
	// isolate: components attach to the active rustup toolchain).
	MethodRustup InstallMethod = "rustup"
	// MethodBinary downloads and verifies an official release binary. Reserved
	// for tools with no mise/registry path; unused until the catalog needs it.
	MethodBinary InstallMethod = "binary"
	// MethodJDTLS downloads and unpacks the Eclipse JDT language server.
	MethodJDTLS InstallMethod = "jdtls"
	// MethodLombok downloads the Lombok jar.
	MethodLombok InstallMethod = "lombok"
	// MethodVSIX downloads and unzips a VS Code marketplace extension.
	MethodVSIX InstallMethod = "vsix"
	// MethodDetect never installs; it only probes PATH and prints Hint when
	// missing, for tools that ship through channels karya must not mutate (LLVM).
	MethodDetect InstallMethod = "detect"
)

// UpdateStrategy describes how a tool is updated, so the update manager can act
// on each independently without touching unrelated tools.
type UpdateStrategy string

const (
	// UpdateLatest reinstalls the tool at its latest version (LSPs, formatters
	// installed via uv/npm/go that track "latest").
	UpdateLatest UpdateStrategy = "latest"
	// UpdateMise upgrades a mise-provisioned tool via mise.
	UpdateMise UpdateStrategy = "mise"
	// UpdatePinned means the version is fixed in the catalog; updating means
	// bumping the catalog entry (jdtls, lombok, VSIX adapters).
	UpdatePinned UpdateStrategy = "pinned"
	// UpdateManual means karya does not update it (detect tools, rustup
	// components managed by the user's rustup).
	UpdateManual UpdateStrategy = "manual"
)

// LocationKind selects which on-disk category directory a tool installs into
// (see config.Paths). Grouping installs by role keeps binaries from scattering
// across unrelated directories and makes per-category update/repair tractable.
type LocationKind string

const (
	// LocCore is the shared prefix for core runtimes/infra and always-on servers.
	LocCore LocationKind = "core"
	// LocDocs is the prefix for documentation tools (prettier, markdownlint, …).
	LocDocs LocationKind = "docs"
	// LocLang is a per-language tool prefix; InstallLocation.Lang names it.
	LocLang LocationKind = "lang"
	// LocRuntime is the language-runtime area (mise-managed; see config.Paths).
	LocRuntime LocationKind = "runtime"
)

// InstallLocation names where a tool's binaries/artifacts live on disk.
type InstallLocation struct {
	Kind LocationKind
	// Lang is the language name when Kind == LocLang (e.g. "go", "java").
	Lang string
}

// Core, Docs, Runtime locations and the per-language Lang constructor keep
// catalog entries terse and prevent typos in the free-form language field.
func Core() InstallLocation      { return InstallLocation{Kind: LocCore} }
func Docs() InstallLocation      { return InstallLocation{Kind: LocDocs} }
func RuntimeAt() InstallLocation { return InstallLocation{Kind: LocRuntime} }
func Lang(name string) InstallLocation {
	return InstallLocation{Kind: LocLang, Lang: name}
}

// HealthCheck describes how to validate a tool. Zero value means the default:
// run the executable with "--version". Probe overrides the existence check for
// artifact-only tools (e.g. a jar or a directory) that have no runnable binary.
type HealthCheck struct {
	// VersionArgs are the args that print the tool's version; nil means
	// {"--version"}.
	VersionArgs []string
	// Probe, when set, is an artifact/dir path (relative to the tool's install
	// dir) whose existence signals the tool is present.
	Probe string
}

// Tool is the complete metadata for one managed tool — the single record the
// registry, resolver, installer, health checker, and update manager all read.
type Tool struct {
	// ID is the stable lookup key (e.g. "gopls", "python-runtime", "ripgrep").
	ID string
	// Name is the human-readable label shown in menus and diagnostics.
	Name string
	// Category is the tool's role (see Category).
	Category Category
	// Method is how the tool is installed (see InstallMethod).
	Method InstallMethod
	// Executable is the command name that resolves the tool on PATH, or "" for
	// artifact-only tools (e.g. lombok.jar) identified by Artifact instead.
	Executable string
	// Artifact, when set, is an install-dir-relative path whose existence signals
	// an installed non-PATH payload (e.g. "lombok.jar", "java-debug").
	Artifact string
	// Pkg is the method-specific identifier: mise/uv/npm registry key, Go module
	// path, rustup component, or a download URL template.
	Pkg string
	// Version pins the tool; "" means latest.
	Version string
	// Dependencies are tool IDs that must be installed first (e.g. jdtls depends
	// on java-runtime; gopls depends on go-runtime).
	Dependencies []string
	// Update is how the tool is updated (see UpdateStrategy).
	Update UpdateStrategy
	// Location selects the tool's on-disk category directory.
	Location InstallLocation
	// Hint is the manual-install suggestion printed for a missing MethodDetect
	// tool (karya will not install it, to preserve isolation).
	Hint string
	// Health describes how to validate the tool (see HealthCheck).
	Health HealthCheck
	// Essential marks a tool karya's core cannot run without (tmux, Neovim); a
	// failure to provide one is a hard error rather than a warning.
	Essential bool
}
