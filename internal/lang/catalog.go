// Package lang manages which programming languages and runtime versions karya
// configures, and generates an isolated mise config from that selection.
//
// It is the karya-native successor to the dotfiles language selector, but obeys
// the isolation model (PLAN.md §2, §6.4): the selection lives at
// config.Paths.LanguagesFile and the generated mise config, runtimes, and shims
// stay entirely inside the karya prefix. The user's global mise is never read or
// written.
//
// The design separates pure, deterministic logic (version dedup, selection
// parsing, mise-config rendering) — unit-tested with no side effects — from the
// thin layer that shells out to mise for version discovery and installation.
package lang

// DedupMode controls how a language's remote versions are collapsed into the
// short specifiers offered to the user. mise resolves a major or major.minor
// specifier to the latest matching patch at install time, so we never store or
// offer full patch versions.
type DedupMode string

const (
	// DedupMajor keeps one entry per major number (e.g. "21" from "21.0.2").
	DedupMajor DedupMode = "major"
	// DedupMinor keeps one entry per major.minor (e.g. "3.14" from "3.14.5").
	DedupMinor DedupMode = "minor"
	// DedupNone means the language has no managed runtime (system toolchain).
	DedupNone DedupMode = "none"
)

// Language describes a language karya can configure and how to discover its
// runtime versions via mise.
type Language struct {
	// Name is the canonical identifier stored in languages.local.
	Name string
	// Display is the human-readable label shown in menus.
	Display string
	// MiseTool is the mise tool name used for version discovery/install, or
	// empty when the language uses the system toolchain (System == true).
	MiseTool string
	// Fallback is the version recorded when mise is unavailable, and the value
	// stored for system-toolchain languages.
	Fallback string
	// Dedup controls remote-version collapsing (see DedupMode).
	Dedup DedupMode
	// System is true for languages with no managed runtime (e.g. C/C++), which
	// always resolve to Fallback and are skipped in mise's [tools] section.
	System bool
}

// Catalog is the ordered set of languages karya can configure. The order is the
// menu order and mirrors the languages the embedded Neovim config supports.
var Catalog = []Language{
	{Name: "python", Display: "Python", MiseTool: "python", Fallback: "3.14", Dedup: DedupMinor},
	{Name: "java", Display: "Java", MiseTool: "java", Fallback: "25", Dedup: DedupMajor},
	{Name: "typescript", Display: "TypeScript/JS", MiseTool: "node", Fallback: "24", Dedup: DedupMajor},
	{Name: "go", Display: "Go", MiseTool: "go", Fallback: "1.26", Dedup: DedupMinor},
	{Name: "cpp", Display: "C/C++", Fallback: "system", Dedup: DedupNone, System: true},
	{Name: "rust", Display: "Rust", MiseTool: "rust", Fallback: "1.97", Dedup: DedupMinor},
}

// Find returns the catalog entry for a language name (or alias), and whether it
// was found. Aliases mirror the ones project scaffolding accepts.
func Find(name string) (Language, bool) {
	canonical, ok := aliases[normalize(name)]
	if !ok {
		return Language{}, false
	}
	for _, l := range Catalog {
		if l.Name == canonical {
			return l, true
		}
	}
	return Language{}, false
}

// Names returns the canonical language names in catalog order (for usage text).
func Names() []string {
	out := make([]string, len(Catalog))
	for i, l := range Catalog {
		out[i] = l.Name
	}
	return out
}

// aliases maps user-facing names to canonical catalog names.
var aliases = map[string]string{
	"python":     "python",
	"py":         "python",
	"java":       "java",
	"typescript": "typescript",
	"ts":         "typescript",
	"js":         "typescript",
	"javascript": "typescript",
	"node":       "typescript",
	"go":         "go",
	"golang":     "go",
	"cpp":        "cpp",
	"c":          "cpp",
	"c++":        "cpp",
	"rust":       "rust",
	"rs":         "rust",
}

func normalize(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			out = append(out, r+('a'-'A'))
		case r == ' ' || r == '\t' || r == '\n':
			// trim whitespace
		default:
			out = append(out, r)
		}
	}
	return string(out)
}
