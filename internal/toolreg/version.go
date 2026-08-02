package toolreg

import (
	"os/exec"
	"strconv"
	"strings"
)

// VersionInfo is a tool's version/update snapshot: the installed version, the
// latest available, and whether an update exists. It is the data `karya tool`
// and `karya doctor --check-updates` render.
type VersionInfo struct {
	ID              string
	Installed       string
	Latest          string
	UpdateAvailable bool
}

// VersionManager reports which managed tools have updates available. It is
// mise-centric: a single `mise outdated` call covers the runtimes and every
// mise-provisioned CLI tool (the majority of the catalog); tools installed by
// other methods report no update info rather than incurring a probe each.
type VersionManager struct {
	reg *Registry
	// outdated returns mise's outdated map (mise tool key -> current/latest);
	// injectable so the manager can be exercised hermetically.
	outdated func() map[string]miseVersions
}

// miseVersions is the current/latest pair mise reports for an outdated tool.
type miseVersions struct{ current, latest string }

// NewVersionManager builds a manager that queries mise with the given env (which
// must pin mise to karya's prefix, e.g. config.Paths.MiseEnv appended to the
// process environment).
func NewVersionManager(reg *Registry, env []string) *VersionManager {
	return &VersionManager{reg: reg, outdated: func() map[string]miseVersions { return miseOutdated(env) }}
}

// Query returns version/update info for the given tool IDs. Unknown IDs are
// skipped. Update detection is available for mise-provisioned tools; others come
// back with UpdateAvailable=false and no Latest.
func (vm *VersionManager) Query(ids []string) []VersionInfo {
	outdated := vm.outdated()
	out := make([]VersionInfo, 0, len(ids))
	for _, id := range ids {
		t, ok := vm.reg.Get(id)
		if !ok {
			continue
		}
		vi := VersionInfo{ID: id}
		if t.Method == MethodMise {
			if o, ok := outdated[t.Pkg]; ok {
				vi.Installed = o.current
				vi.Latest = o.latest
				vi.UpdateAvailable = o.latest != "" &&
					(o.current == "" || compareVersions(o.current, o.latest) < 0)
			}
		}
		out = append(out, vi)
	}
	return out
}

// Updates returns only the tools from the given IDs that have an update
// available — the compact set `doctor` surfaces.
func (vm *VersionManager) Updates(ids []string) []VersionInfo {
	var out []VersionInfo
	for _, vi := range vm.Query(ids) {
		if vi.UpdateAvailable {
			out = append(out, vi)
		}
	}
	return out
}

// miseOutdated runs `mise outdated` and parses each row into current/latest,
// keyed by the mise tool name. It is best-effort: any parse failure yields an
// empty/partial map rather than an error, so a mise output-format change degrades
// to "latest unknown" instead of breaking the command.
func miseOutdated(env []string) map[string]miseVersions {
	mise, err := exec.LookPath("mise")
	if err != nil {
		return nil
	}
	cmd := exec.Command(mise, "outdated")
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	res := make(map[string]miseVersions)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := fields[0]
		if strings.EqualFold(name, "Tool") || strings.EqualFold(name, "Plugin") || strings.EqualFold(name, "Name") {
			continue // header row
		}
		// mise prints Requested/Current/Latest as version-like columns (plus a
		// non-version Source path); the last two version-like fields are
		// current then latest.
		var vers []string
		for _, f := range fields[1:] {
			if looksLikeVersion(f) {
				vers = append(vers, f)
			}
		}
		if len(vers) == 0 {
			continue
		}
		mv := miseVersions{latest: vers[len(vers)-1]}
		if len(vers) >= 2 {
			mv.current = vers[len(vers)-2]
		}
		res[name] = mv
	}
	return res
}

// looksLikeVersion reports whether s begins with a digit (a cheap heuristic that
// separates version columns from a config-path Source column).
func looksLikeVersion(s string) bool {
	return len(s) > 0 && s[0] >= '0' && s[0] <= '9'
}

// compareVersions compares two dotted versions field-by-field numerically,
// ignoring a leading "v" and any pre-release/build suffix on each field. It
// returns -1, 0, or 1 for a<b, a==b, a>b. (Copied from internal/update to keep
// toolreg free of that package's network dependencies.)
func compareVersions(a, b string) int {
	fa, fb := versionFields(a), versionFields(b)
	for i := 0; i < len(fa) || i < len(fb); i++ {
		var x, y int
		if i < len(fa) {
			x = fa[i]
		}
		if i < len(fb) {
			y = fb[i]
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

// versionFields splits a version like "v1.2.3-rc1" into [1, 2, 3], parsing the
// leading integer of each dot-separated component.
func versionFields(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		digits := p
		for i, r := range p {
			if r < '0' || r > '9' {
				digits = p[:i]
				break
			}
		}
		n, _ := strconv.Atoi(digits)
		out = append(out, n)
	}
	return out
}
