// Package tools installs the LSP servers, formatters, and debug adapters karya's
// editor needs — into karya's own tool prefix, never into Homebrew or the user's
// global environment (PLAN.md §2, §6.4).
//
// The plan (which tools a given language selection requires) is pure and
// unit-tested; installation is a thin, detect-first side-effect layer: a tool
// already resolvable on PATH is left alone, and anything karya installs lands
// under config.Paths.ToolsDir so uninstall is a single directory removal.
package tools

// Kind is how a tool is installed. Each kind maps to one installer method.
type Kind string

const (
	// KindUV installs a Python tool via `uv tool install` into karya's bin.
	KindUV Kind = "uv"
	// KindNPM installs a Node package with an isolated npm --prefix.
	KindNPM Kind = "npm"
	// KindGo installs a Go tool with GOBIN set to karya's bin.
	KindGo Kind = "go"
	// KindRustup adds a rustup component (rust-analyzer/rustfmt/clippy).
	KindRustup Kind = "rustup"
	// KindDetect only checks whether a tool is already on PATH; if it is missing
	// karya prints Hint rather than installing, because these servers ship
	// through channels (LLVM, distro packages) karya must not mutate under the
	// isolation model (PLAN.md §2).
	KindDetect Kind = "detect"
	// KindMise installs a tool from mise's default registry into karya's isolated
	// prefix. Unlike KindDetect it never touches Homebrew: the tool is declared in
	// karya's generated mise config (so its shim resolves) and provisioned by the
	// same vendored mise that installs karya's runtimes. Pkg is the mise registry
	// key (e.g. "taplo").
	KindMise Kind = "mise"
	// KindJDTLS downloads and unpacks the Eclipse JDT language server.
	KindJDTLS Kind = "jdtls"
	// KindLombok downloads the Lombok jar.
	KindLombok Kind = "lombok"
	// KindVSIX downloads and unzips a VS Code marketplace extension (Java DAP).
	KindVSIX Kind = "vsix"
)

// ToolSpec describes one installable tool. Bin is the command whose presence on
// PATH means the tool is already available (installation is skipped). Artifact,
// when set, is a path relative to ToolsDir whose existence signals an installed
// non-PATH artifact (e.g. lombok.jar). Pkg is the kind-specific package name,
// module path, or URL template; Version pins it ("" means latest).
type ToolSpec struct {
	Name     string
	Bin      string
	Artifact string
	Kind     Kind
	Pkg      string
	Version  string
	// Hint is the manual-install suggestion printed for a missing KindDetect
	// tool (karya will not install it, to preserve isolation).
	Hint string
}

// alwaysOn are the language servers karya provides regardless of the selected
// languages: config/markup/shell support that every project benefits from
// (PLAN.md §6.4). npm servers install into karya's isolated npm prefix; the
// KindMise servers install from mise's registry into the same isolated prefix.
var alwaysOn = []ToolSpec{
	{Name: "lua-language-server", Bin: "lua-language-server", Kind: KindMise, Pkg: "lua-language-server"},
	{Name: "json/html/css (vscode-langservers-extracted)", Bin: "vscode-json-language-server", Kind: KindNPM, Pkg: "vscode-langservers-extracted"},
	{Name: "yaml-language-server", Bin: "yaml-language-server", Kind: KindNPM, Pkg: "yaml-language-server"},
	{Name: "bash-language-server", Bin: "bash-language-server", Kind: KindNPM, Pkg: "bash-language-server"},
	{Name: "taplo (TOML)", Bin: "taplo", Kind: KindMise, Pkg: "taplo"},
	{Name: "marksman (Markdown)", Bin: "marksman", Kind: KindMise, Pkg: "marksman"},
}

// perLanguage maps a canonical language name to the tools it needs. LSP/formatter
// versions track "latest" (they update independently of the runtime); pinned
// download artifacts use known-good versions. Everything installs into karya's
// isolated prefix.
var perLanguage = map[string][]ToolSpec{
	"python": {
		{Name: "basedpyright", Bin: "basedpyright", Kind: KindUV, Pkg: "basedpyright"},
		{Name: "ruff", Bin: "ruff", Kind: KindUV, Pkg: "ruff"},
		{Name: "debugpy", Bin: "debugpy", Kind: KindUV, Pkg: "debugpy"},
	},
	"typescript": {
		{Name: "typescript-language-server", Bin: "typescript-language-server", Kind: KindNPM, Pkg: "typescript-language-server"},
		{Name: "typescript", Bin: "tsc", Kind: KindNPM, Pkg: "typescript"},
		{Name: "prettier", Bin: "prettier", Kind: KindNPM, Pkg: "prettier"},
	},
	"go": {
		{Name: "gopls", Bin: "gopls", Kind: KindGo, Pkg: "golang.org/x/tools/gopls@latest"},
		{Name: "goimports", Bin: "goimports", Kind: KindGo, Pkg: "golang.org/x/tools/cmd/goimports@latest"},
		{Name: "delve", Bin: "dlv", Kind: KindGo, Pkg: "github.com/go-delve/delve/cmd/dlv@latest"},
	},
	"cpp": {
		{Name: "clangd", Bin: "clangd", Kind: KindDetect, Hint: "install LLVM/clang (brew install llvm, or your distro's clang package)"},
	},
	"rust": {
		{Name: "rust-analyzer", Bin: "rust-analyzer", Kind: KindRustup, Pkg: "rust-analyzer"},
		{Name: "rustfmt", Bin: "rustfmt", Kind: KindRustup, Pkg: "rustfmt"},
		{Name: "clippy", Bin: "cargo-clippy", Kind: KindRustup, Pkg: "clippy"},
	},
	"java": {
		{Name: "jdtls", Bin: "jdtls", Kind: KindJDTLS, Version: "1.44.0"},
		{Name: "lombok", Artifact: "lombok.jar", Kind: KindLombok, Version: "1.18.36"},
		{
			Name:     "java-debug",
			Artifact: "java-debug",
			Kind:     KindVSIX,
			Version:  "0.59.0",
			Pkg:      "https://marketplace.visualstudio.com/_apis/public/gallery/publishers/vscjava/vsextensions/vscode-java-debug/{version}/vspackage",
		},
		{
			Name:     "java-test",
			Artifact: "java-test",
			Kind:     KindVSIX,
			Version:  "0.46.0",
			Pkg:      "https://marketplace.visualstudio.com/_apis/public/gallery/publishers/vscjava/vsextensions/vscode-java-test/{version}/vspackage",
		},
	},
}

// Plan returns the tools to install for the given selected languages: the
// always-on servers first, then each selected language's tools in catalog order.
// Unknown language names are ignored. The result is deterministic.
func Plan(langs []string) []ToolSpec {
	plan := make([]ToolSpec, 0, len(alwaysOn)+len(langs)*3)
	plan = append(plan, alwaysOn...)
	for _, lang := range langs {
		plan = append(plan, perLanguage[lang]...)
	}
	return plan
}

// PlanFor returns just the tools for a single language (no always-on servers),
// used when adding one language interactively.
func PlanFor(lang string) []ToolSpec {
	return perLanguage[lang]
}

// AlwaysOnMise returns the always-on servers provisioned through the vendored
// mise. The CLI declares these in karya's generated mise config so their shims
// resolve and `mise install` provisions them into the isolated prefix.
func AlwaysOnMise() []ToolSpec {
	var out []ToolSpec
	for _, s := range alwaysOn {
		if s.Kind == KindMise {
			out = append(out, s)
		}
	}
	return out
}
