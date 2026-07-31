package cli

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/drjzlyan/karya/internal/editor"
	"github.com/drjzlyan/karya/internal/tools"
	"github.com/drjzlyan/karya/internal/update"
	"github.com/drjzlyan/karya/internal/version"
)

// repoSlug is the GitHub "owner/name" karya self-updates from.
const repoSlug = "drjzlyan/karya"

// cmdInstall performs karya's isolated, non-destructive first-run setup: it
// extracts the embedded configs (done by newApp), applies the current language
// selection (regenerating the isolated mise config and installing runtimes +
// LSP/formatter/adapter tools into the karya prefix), and syncs the editor
// plugins. It changes no user settings; the only opt-in touch-point is the
// printed `karya shellenv` hint.
func cmdInstall(args []string) int {
	a, err := newApp()
	if err != nil {
		return fail(err)
	}

	// Bootstrap karya's own runtime into its isolated prefix: the vendored mise,
	// the tmux + Neovim core, and the language toolchain managers (node, go, rust,
	// uv). On a fresh machine this is what makes the single binary self-contained;
	// on a machine that already has these it is a fast no-op. A failure here is not
	// fatal — the config is still extracted and any tooling that can install will.
	if err := tools.EnsureToolchains(a.paths, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}

	// Apply whatever languages are already selected (empty on a truly fresh
	// machine, which still installs the always-on servers). This regenerates the
	// isolated mise config and installs runtimes + tools without touching Homebrew
	// or the user's global mise.
	sel, err := loadSelection(a)
	if err != nil {
		return fail(err)
	}
	// Onboard a fresh, interactive install straight into language selection so the
	// chosen languages' tools install in this same pass. Non-interactive installs
	// (CI, brew) skip the picker and get the pointer printed below.
	if sel.Empty() && isTerminal(os.Stdin) {
		fmt.Println("\nLet's set up your languages (Enter to skip and do it later with `karya lang`).")
		selectLanguages(a, sel)
	}
	if code := applySelection(a, sel); code != 0 {
		return code
	}

	// Bootstrap the editor's plugins up front so the first launch is instant. A
	// missing nvim is not fatal — plugins bootstrap lazily on first editor launch.
	if err := editor.SyncPlugins(a.env); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}

	fmt.Println("\nkarya is installed (fully isolated; no user settings changed).")
	if sel.Empty() {
		fmt.Println("No languages selected yet — run `karya lang` to add them.")
	}
	fmt.Println("Optional shell integration (adds karya to PATH + sets $EDITOR):")
	fmt.Println(`  eval "$(karya shellenv)"`)
	fmt.Println("Launch the IDE with: karya")
	return 0
}

// cmdUpdate self-updates karya from GitHub Releases. With --check it only reports
// whether a newer release exists. Otherwise it downloads the platform archive and
// checksums, verifies the SHA-256, atomically replaces the running binary, then
// re-runs `install` with the freshly installed binary so the new embedded configs,
// tools, and editor plugins are refreshed.
func cmdUpdate(args []string) int {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	check := fs.Bool("check", false, "only report whether an update is available")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	a, err := newApp()
	if err != nil {
		return fail(err)
	}
	u := update.Updater{
		Repo:    repoSlug,
		Current: version.Version,
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
	}
	rel, err := u.Latest()
	if err != nil {
		return fail(err)
	}
	if !u.UpdateAvailable(rel) {
		fmt.Printf("karya is up to date (%s).\n", version.Version)
		return 0
	}
	if *check {
		fmt.Printf("Update available: %s (current %s).\nRun `karya update` to install it.\n", rel.Tag, version.Version)
		return 0
	}

	fmt.Printf("Updating karya %s → %s...\n", version.Version, rel.Tag)
	bin, err := u.FetchBinary(rel)
	if err != nil {
		return fail(err)
	}
	if err := update.Apply(a.bin, bin); err != nil {
		return fail(err)
	}
	fmt.Printf("Installed %s at %s\n", rel.Tag, a.bin)

	// Finalize with the NEW binary so the just-shipped embedded configs are what
	// get extracted, and tools/plugins refresh against them.
	fmt.Println("Refreshing configs, tools, and editor plugins...")
	cmd := exec.Command(a.bin, "install")
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: post-update refresh failed: %v\n", err)
		fmt.Fprintln(os.Stderr, "The binary was updated; run `karya install` to finish the refresh.")
		return 1
	}
	return 0
}

// cmdUninstall removes everything karya created — the karya prefix directories and
// the karya binary — and nothing else. It prompts for confirmation unless -y is
// given. It never touches the user's own config, Homebrew, or global mise.
func cmdUninstall(args []string) int {
	fs := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	yes := fs.Bool("y", false, "do not prompt for confirmation")
	fs.BoolVar(yes, "yes", false, "do not prompt for confirmation")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	a, err := newApp()
	if err != nil {
		return fail(err)
	}

	// Only karya-owned directories, plus the karya binary itself (its parent
	// ~/.local/bin is shared, so we remove the file, not the dir).
	dirs := []string{a.paths.Config, a.paths.Data, a.paths.State, a.paths.Cache}
	fmt.Println("This will remove all karya data and the karya binary:")
	for _, d := range dirs {
		fmt.Printf("  %s\n", d)
	}
	fmt.Printf("  %s\n", a.bin)
	fmt.Println("Your own config, Homebrew, and global mise are untouched.")

	if !*yes && !confirm(os.Stdin, "Continue?") {
		fmt.Println("Aborted.")
		return 0
	}

	for _, d := range dirs {
		if err := os.RemoveAll(d); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not remove %s: %v\n", d, err)
		}
	}
	if err := os.Remove(a.bin); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "warning: could not remove %s: %v\n", a.bin, err)
	}
	fmt.Println("karya uninstalled.")
	return 0
}

// cmdShellenv prints the opt-in shell integration for `eval "$(karya shellenv)"`.
func cmdShellenv(args []string) int {
	a, err := newApp()
	if err != nil {
		return fail(err)
	}
	fmt.Print(a.paths.ShellEnv(a.bin))
	return 0
}

// confirm asks a yes/no question, reading the answer from r and defaulting to no
// (on empty input, EOF, or anything other than y/yes).
func confirm(r io.Reader, prompt string) bool {
	fmt.Printf("%s [y/N] ", prompt)
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}
