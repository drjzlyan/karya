package toolreg

// registry is the single, ordered source of truth for every tool karya manages.
// It folds three formerly separate lists into one metadata table:
//
//   - internal/tools/bootstrap.go — coreTools (tmux, Neovim) and toolchainTools
//     (node, go, rust, uv), here modeled as core infra + language runtimes.
//   - internal/lang/catalog.go — the managed language runtimes (python, java,
//     node, go, rust; C/C++ is a system toolchain with no runtime entry).
//   - internal/tools/catalog.go — the always-on servers and per-language
//     LSP/formatter/linter/debugger tools.
//
// Order is intentional (infra → runtimes → always-on → per-language) and is the
// deterministic default order for planning and diagnostics. IDs are stable keys;
// changing one is a breaking change to state files that reference it.
//
// The data is expressed as Go literals rather than an embedded TOML/JSON catalog
// so karya keeps its zero-external-dependency core (AGENT.md); the table stays
// pure and is validated by table tests.
var registry = []Tool{
	// ── Core infra (was tools/bootstrap.go coreTools) ──────────────────────────
	{
		ID: "tmux", Name: "tmux", Category: CLIUtility, Method: MethodMise,
		Executable: "tmux", Pkg: "tmux", Update: UpdateMise, Location: Core(),
		Essential: true,
	},
	{
		ID: "neovim", Name: "Neovim", Category: CLIUtility, Method: MethodMise,
		Executable: "nvim", Pkg: "neovim", Update: UpdateMise, Location: Core(),
		Essential: true,
	},
	{
		ID: "uv", Name: "uv (Python)", Category: BuildTool, Method: MethodMise,
		Executable: "uv", Pkg: "uv", Update: UpdateMise, Location: Core(),
	},

	// ── Language runtimes (was lang/catalog.go + bootstrap.go toolchainTools) ───
	// C/C++ is a system toolchain and has no managed runtime entry.
	{
		ID: "python-runtime", Name: "Python runtime", Category: Runtime, Method: MethodMise,
		Executable: "python", Pkg: "python", Update: UpdateMise, Location: RuntimeAt(),
	},
	{
		ID: "java-runtime", Name: "Java runtime", Category: Runtime, Method: MethodMise,
		Executable: "java", Pkg: "java", Update: UpdateMise, Location: RuntimeAt(),
	},
	{
		ID: "node-runtime", Name: "Node.js runtime (npm)", Category: Runtime, Method: MethodMise,
		Executable: "node", Pkg: "node", Update: UpdateMise, Location: RuntimeAt(),
	},
	{
		ID: "go-runtime", Name: "Go toolchain", Category: Runtime, Method: MethodMise,
		Executable: "go", Pkg: "go", Update: UpdateMise, Location: RuntimeAt(),
	},
	{
		ID: "rust-runtime", Name: "Rust toolchain", Category: Runtime, Method: MethodMise,
		Executable: "cargo", Pkg: "rust", Update: UpdateMise, Location: RuntimeAt(),
	},

	// ── Always-on servers (was tools/catalog.go alwaysOn) ──────────────────────
	{
		ID: "lua-language-server", Name: "lua-language-server", Category: LanguageServer,
		Method: MethodMise, Executable: "lua-language-server", Pkg: "lua-language-server",
		Update: UpdateMise, Location: Core(),
	},
	{
		ID: "vscode-langservers-extracted", Name: "json/html/css (vscode-langservers-extracted)",
		Category: LanguageServer, Method: MethodNPM, Executable: "vscode-json-language-server",
		Pkg: "vscode-langservers-extracted", Update: UpdateLatest, Location: Core(),
		Dependencies: []string{"node-runtime"},
	},
	{
		ID: "yaml-language-server", Name: "yaml-language-server", Category: LanguageServer,
		Method: MethodNPM, Executable: "yaml-language-server", Pkg: "yaml-language-server",
		Update: UpdateLatest, Location: Core(), Dependencies: []string{"node-runtime"},
	},
	{
		ID: "bash-language-server", Name: "bash-language-server", Category: LanguageServer,
		Method: MethodNPM, Executable: "bash-language-server", Pkg: "bash-language-server",
		Update: UpdateLatest, Location: Core(), Dependencies: []string{"node-runtime"},
	},
	{
		ID: "taplo", Name: "taplo (TOML)", Category: Formatter, Method: MethodMise,
		Executable: "taplo", Pkg: "taplo", Update: UpdateMise, Location: Core(),
	},
	{
		ID: "marksman", Name: "marksman (Markdown)", Category: LanguageServer, Method: MethodMise,
		Executable: "marksman", Pkg: "marksman", Update: UpdateMise, Location: Core(),
	},

	// ── Python (was perLanguage["python"]) ─────────────────────────────────────
	{
		ID: "basedpyright", Name: "basedpyright", Category: LanguageServer, Method: MethodUV,
		Executable: "basedpyright", Pkg: "basedpyright", Update: UpdateLatest,
		Location: Lang("python"), Dependencies: []string{"uv"},
	},
	{
		ID: "ruff", Name: "ruff", Category: Linter, Method: MethodUV,
		Executable: "ruff", Pkg: "ruff", Update: UpdateLatest,
		Location: Lang("python"), Dependencies: []string{"uv"},
	},
	{
		ID: "debugpy", Name: "debugpy", Category: Debugger, Method: MethodUV,
		Executable: "debugpy", Pkg: "debugpy", Update: UpdateLatest,
		Location: Lang("python"), Dependencies: []string{"uv"},
	},

	// ── TypeScript/JS (was perLanguage["typescript"]) ──────────────────────────
	{
		ID: "typescript-language-server", Name: "typescript-language-server",
		Category: LanguageServer, Method: MethodNPM, Executable: "typescript-language-server",
		Pkg: "typescript-language-server", Update: UpdateLatest, Location: Lang("typescript"),
		Dependencies: []string{"node-runtime"},
	},
	{
		ID: "typescript", Name: "typescript", Category: BuildTool, Method: MethodNPM,
		Executable: "tsc", Pkg: "typescript", Update: UpdateLatest, Location: Lang("typescript"),
		Dependencies: []string{"node-runtime"},
	},
	{
		ID: "prettier", Name: "prettier", Category: Formatter, Method: MethodNPM,
		Executable: "prettier", Pkg: "prettier", Update: UpdateLatest, Location: Lang("typescript"),
		Dependencies: []string{"node-runtime"},
	},

	// ── Go (was perLanguage["go"]) ─────────────────────────────────────────────
	{
		ID: "gopls", Name: "gopls", Category: LanguageServer, Method: MethodGo,
		Executable: "gopls", Pkg: "golang.org/x/tools/gopls@latest", Update: UpdateLatest,
		Location: Lang("go"), Dependencies: []string{"go-runtime"},
	},
	{
		ID: "goimports", Name: "goimports", Category: Formatter, Method: MethodGo,
		Executable: "goimports", Pkg: "golang.org/x/tools/cmd/goimports@latest", Update: UpdateLatest,
		Location: Lang("go"), Dependencies: []string{"go-runtime"},
	},
	{
		ID: "delve", Name: "delve", Category: Debugger, Method: MethodGo,
		Executable: "dlv", Pkg: "github.com/go-delve/delve/cmd/dlv@latest", Update: UpdateLatest,
		Location: Lang("go"), Dependencies: []string{"go-runtime"},
	},

	// ── C/C++ (was perLanguage["cpp"]) ─────────────────────────────────────────
	// clangd ships with LLVM, which karya cannot install into its isolated prefix,
	// so it stays detect-only; the build tools and formatter come from mise.
	{
		ID: "clangd", Name: "clangd", Category: LanguageServer, Method: MethodDetect,
		Executable: "clangd", Update: UpdateManual, Location: Lang("cpp"),
		Dependencies: []string{"codelldb"},
		Hint:         "install LLVM/clang (brew install llvm, or your distro's clang package)",
	},
	{ID: "cmake", Name: "CMake", Category: BuildTool, Method: MethodMise, Executable: "cmake", Pkg: "cmake", Update: UpdateMise, Location: Lang("cpp")},
	{ID: "ninja", Name: "Ninja", Category: BuildTool, Method: MethodMise, Executable: "ninja", Pkg: "ninja", Update: UpdateMise, Location: Lang("cpp")},
	{ID: "clang-format", Name: "clang-format", Category: Formatter, Method: MethodMise, Executable: "clang-format", Pkg: "clang-format", Update: UpdateMise, Location: Lang("cpp")},
	{
		// Shared C/C++ + Rust debug adapter; pulled in as a dependency of clangd and
		// rust-analyzer, installed into the shared prefix.
		ID: "codelldb", Name: "codelldb (LLDB debug adapter)", Category: Debugger, Method: MethodVSIX,
		Artifact: "codelldb", Version: "1.11.5", Update: UpdatePinned, Location: Core(),
		Pkg:    "https://marketplace.visualstudio.com/_apis/public/gallery/publishers/vadimcn/vsextensions/vscode-lldb/{version}/vspackage",
		Health: HealthCheck{Probe: "codelldb"},
	},

	// ── Rust (was perLanguage["rust"]) ─────────────────────────────────────────
	// rust-analyzer installs standalone via mise (no rustup needed). rustfmt and
	// clippy ship with the mise-provisioned Rust toolchain, so they are detected
	// after the runtime and only fall back to `rustup component add` if absent.
	{
		ID: "rust-analyzer", Name: "rust-analyzer", Category: LanguageServer, Method: MethodMise,
		Executable: "rust-analyzer", Pkg: "rust-analyzer", Update: UpdateMise, Location: Lang("rust"),
		Dependencies: []string{"codelldb"},
	},
	{
		ID: "rustfmt", Name: "rustfmt", Category: Formatter, Method: MethodRustup,
		Executable: "rustfmt", Pkg: "rustfmt", Update: UpdateManual, Location: Lang("rust"),
		Dependencies: []string{"rust-runtime"},
	},
	{
		ID: "clippy", Name: "clippy", Category: Linter, Method: MethodRustup,
		Executable: "cargo-clippy", Pkg: "clippy", Update: UpdateManual, Location: Lang("rust"),
		Dependencies: []string{"rust-runtime"},
	},

	// ── Java (was perLanguage["java"]) ─────────────────────────────────────────
	{
		ID: "jdtls", Name: "jdtls", Category: LanguageServer, Method: MethodJDTLS,
		Executable: "jdtls", Version: "1.44.0", Update: UpdatePinned, Location: Lang("java"),
		Dependencies: []string{"java-runtime"},
	},
	{
		ID: "lombok", Name: "lombok", Category: BuildTool, Method: MethodLombok,
		Artifact: "lombok.jar", Version: "1.18.36", Update: UpdatePinned, Location: Lang("java"),
		Dependencies: []string{"java-runtime"}, Health: HealthCheck{Probe: "lombok.jar"},
	},
	{
		ID: "java-debug", Name: "java-debug", Category: Debugger, Method: MethodVSIX,
		Artifact: "java-debug", Version: "0.59.0", Update: UpdatePinned, Location: Lang("java"),
		Pkg:          "https://marketplace.visualstudio.com/_apis/public/gallery/publishers/vscjava/vsextensions/vscode-java-debug/{version}/vspackage",
		Dependencies: []string{"java-runtime"}, Health: HealthCheck{Probe: "java-debug"},
	},
	{
		ID: "java-test", Name: "java-test", Category: Debugger, Method: MethodVSIX,
		Artifact: "java-test", Version: "0.46.0", Update: UpdatePinned, Location: Lang("java"),
		Pkg:          "https://marketplace.visualstudio.com/_apis/public/gallery/publishers/vscjava/vsextensions/vscode-java-test/{version}/vspackage",
		Dependencies: []string{"java-runtime"}, Health: HealthCheck{Probe: "java-test"},
	},
	{ID: "google-java-format", Name: "google-java-format", Category: Formatter, Method: MethodMise, Executable: "google-java-format", Pkg: "google-java-format", Update: UpdateMise, Location: Lang("java")},
	{ID: "maven", Name: "Maven", Category: BuildTool, Method: MethodMise, Executable: "mvn", Pkg: "maven", Update: UpdateMise, Location: Lang("java")},
	{ID: "gradle", Name: "Gradle", Category: BuildTool, Method: MethodMise, Executable: "gradle", Pkg: "gradle", Update: UpdateMise, Location: Lang("java")},

	// ── Core CLI utilities ─────────────────────────────────────────────────────
	// General-purpose tools the IDE (and the user) rely on, provisioned through
	// the vendored mise so karya never assumes a global install. git is the one
	// exception: system git is universal and mise-git is heavy, so it is detected
	// and hinted rather than installed.
	{
		ID: "git", Name: "git", Category: CLIUtility, Method: MethodDetect,
		Executable: "git", Update: UpdateManual, Location: Core(),
		Hint: "install git for version control (most systems ship it)",
	},
	{ID: "gh", Name: "GitHub CLI", Category: CLIUtility, Method: MethodMise, Executable: "gh", Pkg: "gh", Update: UpdateMise, Location: Core()},
	{ID: "jq", Name: "jq", Category: CLIUtility, Method: MethodMise, Executable: "jq", Pkg: "jq", Update: UpdateMise, Location: Core()},
	{ID: "yq", Name: "yq", Category: CLIUtility, Method: MethodMise, Executable: "yq", Pkg: "yq", Update: UpdateMise, Location: Core()},
	{ID: "fd", Name: "fd", Category: CLIUtility, Method: MethodMise, Executable: "fd", Pkg: "fd", Update: UpdateMise, Location: Core()},
	{ID: "ripgrep", Name: "ripgrep", Category: CLIUtility, Method: MethodMise, Executable: "rg", Pkg: "ripgrep", Update: UpdateMise, Location: Core()},
	{ID: "fzf", Name: "fzf", Category: CLIUtility, Method: MethodMise, Executable: "fzf", Pkg: "fzf", Update: UpdateMise, Location: Core()},
	{ID: "bat", Name: "bat", Category: CLIUtility, Method: MethodMise, Executable: "bat", Pkg: "bat", Update: UpdateMise, Location: Core()},
	{ID: "eza", Name: "eza", Category: CLIUtility, Method: MethodMise, Executable: "eza", Pkg: "eza", Update: UpdateMise, Location: Core()},
	{ID: "delta", Name: "delta", Category: CLIUtility, Method: MethodMise, Executable: "delta", Pkg: "delta", Update: UpdateMise, Location: Core()},
	{ID: "tree", Name: "tree", Category: CLIUtility, Method: MethodMise, Executable: "tree", Pkg: "tree", Update: UpdateMise, Location: Core()},
	{ID: "zoxide", Name: "zoxide", Category: CLIUtility, Method: MethodMise, Executable: "zoxide", Pkg: "zoxide", Update: UpdateMise, Location: Core()},
	{ID: "just", Name: "just", Category: BuildTool, Method: MethodMise, Executable: "just", Pkg: "just", Update: UpdateMise, Location: Core()},
	{ID: "watchexec", Name: "watchexec", Category: CLIUtility, Method: MethodMise, Executable: "watchexec", Pkg: "watchexec", Update: UpdateMise, Location: Core()},
	{ID: "hyperfine", Name: "hyperfine", Category: CLIUtility, Method: MethodMise, Executable: "hyperfine", Pkg: "hyperfine", Update: UpdateMise, Location: Core()},
	{ID: "shellcheck", Name: "shellcheck", Category: Linter, Method: MethodMise, Executable: "shellcheck", Pkg: "shellcheck", Update: UpdateMise, Location: Core()},
	{ID: "shfmt", Name: "shfmt", Category: Formatter, Method: MethodMise, Executable: "shfmt", Pkg: "shfmt", Update: UpdateMise, Location: Core()},

	// ── Documentation tools ────────────────────────────────────────────────────
	// prettier and taplo already appear above (shared with language/always-on
	// tooling); these round out documentation formatting and linting.
	{
		ID: "markdownlint", Name: "markdownlint", Category: Linter, Method: MethodNPM,
		Executable: "markdownlint-cli2", Pkg: "markdownlint-cli2", Update: UpdateLatest,
		Location: Docs(), Dependencies: []string{"node-runtime"},
	},
	{
		ID: "yamllint", Name: "yamllint", Category: Linter, Method: MethodUV,
		Executable: "yamllint", Pkg: "yamllint", Update: UpdateLatest,
		Location: Docs(), Dependencies: []string{"uv"},
	},
	{
		ID: "yamlfmt", Name: "yamlfmt", Category: Formatter, Method: MethodMise,
		Executable: "yamlfmt", Pkg: "yamlfmt", Update: UpdateMise, Location: Docs(),
	},
	{
		ID: "jsonlint", Name: "jsonlint", Category: Linter, Method: MethodNPM,
		Executable: "jsonlint", Pkg: "jsonlint", Update: UpdateLatest,
		Location: Docs(), Dependencies: []string{"node-runtime"},
	},
}
