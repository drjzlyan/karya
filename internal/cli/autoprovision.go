package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/drjzlyan/karya/internal/toolreg"
	"github.com/drjzlyan/karya/internal/tools"
)

// autoProvisioner installs a language's editor tooling (LSP server, formatter,
// linter) into karya's isolated prefix on demand, so the embedded editor gets
// language support with zero user action (the hybrid Phase-2 approach: auto-
// detect over the existing mise/tool catalog now, marketplace later).
//
// It implements ide.Provisioner. All installer output goes to a log file — never
// stdout/stderr — because the TUI owns the screen. Work is serialized and
// deduped per language so repeated opens of the same language are cheap no-ops.
type autoProvisioner struct {
	app  *app
	logw io.Writer

	mu   sync.Mutex
	done map[string]bool
}

// newAutoProvisioner builds a provisioner that logs to the karya tools log dir.
func newAutoProvisioner(a *app) *autoProvisioner {
	w := io.Discard
	_ = os.MkdirAll(a.paths.ToolsLogsDir(), 0o755)
	logPath := filepath.Join(a.paths.ToolsLogsDir(), "lsp-provision.log")
	if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
		w = f
	}
	return &autoProvisioner{app: a, logw: w, done: map[string]bool{}}
}

// EnsureLanguage installs the editor tooling for langName if it is not already
// present. It blocks until done and is safe to call from a background goroutine;
// callers get a nil error when the language has no catalog tooling.
func (ap *autoProvisioner) EnsureLanguage(langName string) error {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	if ap.done[langName] {
		return nil
	}
	ids := languageToolIDs(ap.app.reg, langName)
	if len(ids) == 0 {
		ap.done[langName] = true // nothing to do for this language
		return nil
	}
	if ap.app.allResolve(ids) {
		ap.done[langName] = true // already installed
		return nil
	}
	fmt.Fprintf(ap.logw, "\n=== auto-provision %s: %v ===\n", langName, ids)
	if _, err := tools.EnsureMise(ap.app.paths, ap.logw, ap.logw); err != nil {
		return fmt.Errorf("provision %s: %w", langName, err)
	}
	plan, err := ap.app.reg.Plan(ids)
	if err != nil {
		return fmt.Errorf("provision %s: %w", langName, err)
	}
	env := append(os.Environ(), ap.app.env...)
	d := tools.NewLayoutDispatcher(ap.app.paths)
	results := d.Install(plan, tools.Context{Env: env, Out: ap.logw, ErrOut: ap.logw})
	ap.app.refreshToolManifest()
	ap.done[langName] = true
	fmt.Fprintf(ap.logw, "=== auto-provision %s: %s ===\n", langName, tools.Summarize(results))
	return firstInstallErr(results)
}

// languageToolIDs returns the catalog tool IDs providing editor support for a
// language: its language server, formatter, and linter (per-language tools;
// always-on core servers are provisioned by `karya install`).
func languageToolIDs(reg *toolreg.Registry, langName string) []string {
	var ids []string
	for _, t := range reg.All() {
		if t.Location.Kind != toolreg.LocLang || t.Location.Lang != langName {
			continue
		}
		switch t.Category {
		case toolreg.LanguageServer, toolreg.Formatter, toolreg.Linter:
			ids = append(ids, t.ID)
		}
	}
	return ids
}

// firstInstallErr returns the first failed install's error, if any.
func firstInstallErr(results []tools.Result) error {
	for _, r := range results {
		if r.Status == tools.Failed {
			if r.Err != nil {
				return r.Err
			}
			return fmt.Errorf("install failed: %s", r.Tool)
		}
	}
	return nil
}
