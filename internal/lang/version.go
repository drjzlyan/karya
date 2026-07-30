package lang

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// VersionLister discovers the versions a runtime offers. The mise-backed
// implementation is MiseLister; tests inject a fake so version logic stays pure.
type VersionLister interface {
	// ListRemote returns the raw version strings mise reports for a tool, in
	// mise's native order (ascending). It returns an error only for an actual
	// failure to run the query; an unknown tool yields an empty slice, nil.
	ListRemote(tool string) ([]string, error)
}

// prerelease matches version strings karya should never offer or install.
var prerelease = regexp.MustCompile(`(?i)(alpha|beta|rc|dev|pre|nightly|snapshot|ea|jre|[ab][0-9])`)

// AvailableVersions returns the short version specifiers to offer for a
// language, most-recent last. It filters prereleases and collapses versions per
// the language's DedupMode. Java is special-cased to pick the best available
// distribution per major. On any error, or when the lister returns nothing, it
// falls back to the single catalog fallback version so selection always works
// offline.
func AvailableVersions(lister VersionLister, l Language) []string {
	if l.System || l.MiseTool == "" || l.Dedup == DedupNone {
		return []string{l.Fallback}
	}
	raw, err := lister.ListRemote(l.MiseTool)
	if err != nil || len(raw) == 0 {
		return []string{l.Fallback}
	}
	var versions []string
	if l.MiseTool == "java" {
		versions = javaVersions(raw)
	} else {
		versions = DedupVersions(filterStable(raw), l.Dedup)
	}
	if len(versions) == 0 {
		return []string{l.Fallback}
	}
	return versions
}

// filterStable drops prerelease and non-numeric entries, preserving order.
func filterStable(raw []string) []string {
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		v = strings.TrimSpace(v)
		if v == "" || v[0] < '0' || v[0] > '9' {
			continue
		}
		if prerelease.MatchString(v) {
			continue
		}
		out = append(out, v)
	}
	return out
}

// DedupVersions collapses ascending version strings to one short specifier per
// dedup key, preserving ascending order. The first occurrence of each key wins;
// since input is ascending and mise resolves a major/major.minor specifier to
// the latest patch, the short key is all we need.
func DedupVersions(versions []string, mode DedupMode) []string {
	seen := make(map[string]bool)
	var out []string
	for _, v := range versions {
		key := versionKey(v, mode)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	return out
}

// versionKey extracts the dedup key from a version string.
func versionKey(version string, mode DedupMode) string {
	parts := strings.Split(version, ".")
	switch mode {
	case DedupMajor:
		return parts[0]
	case DedupMinor:
		if len(parts) >= 2 {
			return parts[0] + "." + parts[1]
		}
		return parts[0]
	default:
		return version
	}
}

// javaDistros lists the mise java distribution prefixes karya will accept for a
// major, in descending preference. A plain numeric spec (e.g. "21") is always
// preferred and handled separately.
var javaDistros = []string{"temurin", "corretto", "zulu"}

// javaVersions picks, for each major from 8 up to the newest offered, the best
// available java specifier: a plain major if present, else the first matching
// distribution by preference. Result is ascending by major.
func javaVersions(raw []string) []string {
	stable := make([]string, 0, len(raw))
	for _, v := range raw {
		v = strings.TrimSpace(v)
		if v != "" && !prerelease.MatchString(v) {
			stable = append(stable, v)
		}
	}

	maxMajor := 0
	plainMajor := regexp.MustCompile(`^(\d+)\.`)
	for _, v := range stable {
		if m := plainMajor.FindStringSubmatch(v); m != nil {
			if n, _ := strconv.Atoi(m[1]); n > maxMajor {
				maxMajor = n
			}
		}
	}
	if maxMajor < 8 {
		return nil
	}

	var out []string
	for major := 8; major <= maxMajor; major++ {
		if spec := bestJavaSpec(stable, major); spec != "" {
			out = append(out, spec)
		}
	}
	return out
}

// bestJavaSpec returns the preferred available specifier for a java major.
func bestJavaSpec(stable []string, major int) string {
	m := strconv.Itoa(major)
	plain := regexp.MustCompile(`^` + m + `\.`)
	for _, v := range stable {
		if plain.MatchString(v) {
			return m
		}
	}
	for _, distro := range javaDistros {
		want := distro + "-" + m
		re := regexp.MustCompile(`^` + regexp.QuoteMeta(want) + `(\D|$)`)
		for _, v := range stable {
			if re.MatchString(v) {
				return want
			}
		}
	}
	return ""
}

// MiseLister discovers versions by running `mise ls-remote <tool>` under karya's
// isolated mise environment. Env is the environment to run mise with (typically
// os.Environ() plus config.Paths.MiseEnv()).
type MiseLister struct {
	Env []string
}

// ListRemote runs `mise ls-remote <tool>`. If mise is not installed it returns
// no versions (and no error) so callers fall back to the catalog default.
func (m MiseLister) ListRemote(tool string) ([]string, error) {
	if _, err := exec.LookPath("mise"); err != nil {
		return nil, nil
	}
	cmd := exec.Command("mise", "ls-remote", tool)
	if m.Env != nil {
		cmd.Env = m.Env
	}
	out, err := cmd.Output()
	if err != nil {
		return nil, nil // treat query failure as "no versions"; fall back
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			cleaned = append(cleaned, l)
		}
	}
	return cleaned, nil
}

// MiseInstalled reports whether mise is available on PATH.
func MiseInstalled() bool {
	_, err := exec.LookPath("mise")
	return err == nil
}
