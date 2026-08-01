package toolreg

// Profile is an installable bundle of tools (and, for language profiles, the
// runtimes they need). Installing a profile provisions everything required for
// an ecosystem in one step, so users think in terms of "Python" or "Documentation"
// rather than individual packages.
type Profile struct {
	// ID is the stable profile key ("core", "docs", "python", …).
	ID string
	// Name is the human-readable label.
	Name string
	// Tools are the tool IDs the profile installs.
	Tools []string
	// Runtimes are the language names whose runtime the profile requires (feeds
	// the language selection / runtime provisioning). Empty for non-language
	// profiles.
	Runtimes []string
}

// alwaysOnIDs are the servers karya provides regardless of language selection:
// config/markup/shell support every project benefits from. They are installed by
// the baseline setup, not gated behind a profile.
var alwaysOnIDs = []string{
	"lua-language-server",
	"vscode-langservers-extracted",
	"yaml-language-server",
	"bash-language-server",
	"taplo",
	"marksman",
}

// nonLanguageProfiles are the ecosystem-independent bundles. Language profiles
// are derived from the registry (see Profiles) so they never drift from the
// catalog. These two carry explicit membership because they group tools by role
// rather than by a single install location.
var nonLanguageProfiles = []Profile{
	{
		ID:   "core",
		Name: "Core CLI",
		Tools: []string{
			"git", "gh", "jq", "yq", "fd", "ripgrep", "fzf", "bat", "eza",
			"delta", "tree", "zoxide", "just", "watchexec", "hyperfine",
			"shellcheck", "shfmt",
		},
	},
	{
		ID:   "docs",
		Name: "Documentation",
		Tools: []string{
			"prettier", "taplo", "marksman", "markdownlint", "yamllint",
			"yamlfmt", "jsonlint",
		},
	},
}

// languageProfiles are the language names, in a stable order, that get a derived
// profile. System languages (no managed runtime) still get a tools profile.
var languageProfiles = []struct {
	id      string
	name    string
	runtime bool // whether the language has a managed runtime to provision
}{
	{"python", "Python", true},
	{"typescript", "TypeScript/JS", true},
	{"go", "Go", true},
	{"rust", "Rust", true},
	{"java", "Java", true},
	{"cpp", "C/C++", false},
}

// AlwaysOnIDs returns the always-on server tool IDs.
func AlwaysOnIDs() []string { return append([]string(nil), alwaysOnIDs...) }

// LanguageIDs returns the tool IDs for a language, taken from the registry by
// install location so the list is always in sync with the catalog.
func (r *Registry) LanguageIDs(lang string) []string {
	var out []string
	for _, t := range r.all {
		if t.Location.Kind == LocLang && t.Location.Lang == lang {
			out = append(out, t.ID)
		}
	}
	return out
}

// Profiles returns every profile: the core and docs bundles plus one derived
// profile per supported language.
func (r *Registry) Profiles() []Profile {
	out := append([]Profile(nil), nonLanguageProfiles...)
	for _, lp := range languageProfiles {
		p := Profile{ID: lp.id, Name: lp.name, Tools: r.LanguageIDs(lp.id)}
		if lp.runtime {
			p.Runtimes = []string{lp.id}
		}
		out = append(out, p)
	}
	return out
}

// Profile returns the profile with the given ID and whether it was found.
func (r *Registry) Profile(id string) (Profile, bool) {
	for _, p := range r.Profiles() {
		if p.ID == id {
			return p, true
		}
	}
	return Profile{}, false
}
