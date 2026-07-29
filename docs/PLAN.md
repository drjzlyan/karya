# karya — Architecture & Design Plan

> **karya** (कार्य — "work/task") is an AI-first, terminal-based IDE delivered as a
> single self-contained Go binary. It consolidates the author's existing
> [`nvim-config`](../../nvim-config) (the editor) and
> [`dotfiles`](../../dotfiles) (the machine/session tooling) into one program
> that installs, launches, manages, and updates a complete terminal development
> environment — **without touching any of the user's existing settings.**

---

## 1. Product vision

Today the IDE is assembled from many moving parts:

- **Neovim** with a capability-oriented Lua config (LSP, completion, DAP, git,
  tasks, testing, 6 languages, health checks).
- **tmux** driving a 3-pane layout (editor / agent / build+test) plus a git window.
- **Shell scripts** (`dev.sh`, `ide-agent.sh`, `ide-run.sh`, `nvim-edit`,
  `project-init.sh`, `languages.sh`, `install.sh`, `link.sh`, `update.sh`,
  `rebuild.sh`, `doctor.sh`).
- **External tools** (ripgrep, fd, fzf, lazygit, delta, starship, zoxide, uv,
  mise, LSP servers, formatters, debug adapters) installed via Homebrew + mise.

`karya` replaces the shell-script layer and the manual assembly with a single
binary. The coding agent (claude / codex / crush / gemini / aider / copilot) is
promoted from an optional pane to a **first-class, deeply-integrated citizen**.

### Design pillars

1. **Single binary.** One `karya` executable. The Neovim config and tmux config
   are **embedded** in the binary via `go:embed` and extracted on demand.
2. **Zero-impact installation.** karya never edits or symlinks over the user's
   `~/.zshrc`, `~/.tmux.conf`, `~/.gitconfig`, or `~/.config/nvim`. Everything
   lives under karya-owned XDG directories. Uninstall removes only karya.
3. **AI-first.** The agent pane, agent switching, per-project agent memory, and
   context routing are core features, not add-ons.
4. **Self-updating.** `karya update` upgrades the binary, embedded configs,
   managed tools, and editor plugins.
5. **Reuse, don't rewrite.** Neovim remains the editor engine; tmux remains the
   multiplexer. karya orchestrates them.

---

## 2. The isolation model (the most important design decision)

The current `dotfiles` approach **symlinks over** `~/.zshrc`, `~/.tmux.conf`,
`~/.config/nvim`, etc. That is exactly what karya must **not** do. karya achieves
"installation does not impact any other user settings" through strict
namespacing:

| Concern | User's setup (untouched) | karya's isolated equivalent |
|---|---|---|
| Binary | — | `~/.local/bin/karya` (single file) |
| Config | `~/.config/nvim`, `~/.tmux.conf` | `~/.config/karya/**` |
| Data | `~/.local/share/nvim` | `~/.local/share/karya/**` |
| State | `~/.local/state/nvim` | `~/.local/state/karya/**` |
| Cache | `~/.cache/nvim` | `~/.cache/karya/**` |
| Neovim | user's `nvim` config | launched with `NVIM_APPNAME=karya` |
| tmux | user's default socket + `~/.tmux.conf` | dedicated socket `-L karya` + `-f` karya conf |
| Shell | user's `~/.zshrc` | **opt-in** `eval "$(karya shellenv)"` only |
| Tools | user's Homebrew | karya prefix `~/.local/share/karya/tools/bin` (detect-or-install) |

Key mechanisms:

- **`NVIM_APPNAME=karya`** — Neovim natively reads config from
  `~/.config/karya/` and data from `~/.local/share/karya/` when this env var is
  set. The user's own `~/.config/nvim` is never read or written. This is the
  clean, first-class isolation primitive Neovim provides.
- **Dedicated tmux server** — `tmux -L karya -f <karya tmux.conf>` runs on a
  separate socket with its own config. karya's sessions never collide with the
  user's tmux, and the user's `~/.tmux.conf` is never sourced.
- **Opt-in shell integration** — karya works fully as a bare binary on `PATH`.
  Users who want aliases / `EDITOR` routing / PATH additions add a single line:
  `eval "$(karya shellenv)"`. karya never rewrites shell rc files itself.
- **Tool management** — karya prefers tools already on `PATH`; anything it needs
  and cannot find, it installs into its own prefix (never into Homebrew or the
  user's global env). Homebrew is optional, not required.

**Uninstall guarantee:** `karya uninstall` deletes the karya prefix directories
and the binary, and nothing else. No user config is mutated at any point, so
there is nothing to restore.

---

## 3. Feature consolidation map

Every existing capability maps to a karya command or subsystem:

| Existing (dotfiles / nvim-config) | karya replacement |
|---|---|
| `dev.sh` (tmux IDE session) | `karya` / `karya dev [name] [path]` |
| `ide-agent.sh` (switch/next/prev/reset/status/prefs) | `karya agent <subcommand>` |
| `ide-run.sh` (route cmd to build/test pane) | `karya run <cmd>` |
| `bin/nvim-edit` (open file in editor pane) | `karya edit <file> [line]` (also the `$EDITOR`) |
| `project-init.sh` (scaffold python/java/ts/go/cpp/rust) | `karya new <lang> <name> [dir]` |
| `languages.sh` (select langs + versions, gen mise.toml) | `karya lang [list\|add\|remove\|all]` |
| `install.sh` (bootstrap machine) | `karya install` (isolated, non-destructive) |
| `link.sh` / `unlink.sh` (symlink dotfiles) | **removed** — replaced by embedded configs + `NVIM_APPNAME` |
| `update.sh` (upgrade everything) | `karya update` |
| `rebuild.sh` (re-apply after pull) | folded into `karya update` |
| `doctor.sh` + `:DevHealth` | `karya doctor` |
| `nvim-config/**` (Lua editor config) | embedded assets, extracted to `~/.config/karya/nvim` |
| `.tmux.conf` | embedded asset, karya-owned socket + conf |
| `.zshrc` aliases / PATH | `karya shellenv` (opt-in) |
| `starship.toml`, `lazygit`, `ghostty`, `.gitconfig` | embedded/optional; never overwrite user files |
| tmux keybindings (`prefix + A/N/D/P/S`) | wired to `karya agent …` / `karya new` in embedded tmux.conf |

---

## 4. Command surface (CLI)

```
karya                         Launch or attach the IDE session for the cwd
karya dev [name] [path]       Explicit session launch
  -a, --agent <name|none>     Choose the coding agent
  -k, --kill                  Kill an existing session and recreate
  -q, --quit                  Quit/kill the session cleanly

karya agent status            Show current/available agents + saved preference
karya agent switch            Interactive agent picker (in-session)
karya agent next | prev       Cycle agents
karya agent reset             Reset the pane layout (preserves editor)
karya agent prefs | clear     Inspect / clear per-project preference

karya edit <file> [line]      Open a file in the editor pane (used as $EDITOR)
karya run [-d dir] <cmd...>   Run a command in the build/test pane
karya run --focus             Focus the build/test pane

karya new <lang> <name> [dir] Scaffold a project (python|java|typescript|go|cpp|rust)

karya lang                    Interactive language + version selector
karya lang list | add | remove | all

karya install                 Set up karya (isolated): extract configs, fetch tools
karya update [--check]        Self-update binary + configs + tools + editor plugins
karya uninstall               Remove karya entirely (nothing else touched)

karya doctor                  Health check (tools, versions, isolation, LSPs)
karya shellenv                Print opt-in shell integration (eval this)
karya version                 Print version / build info
karya completion <shell>      Shell completions
```

In-session tmux keybindings (embedded `tmux.conf`, prefix `Ctrl-a`), preserved
from the current setup and wired to karya:

| Key | Action | Backed by |
|---|---|---|
| `Ctrl-a A` | Switch agent (prompt) | `karya agent switch` |
| `Ctrl-a N` | Next agent | `karya agent next` |
| `Ctrl-a D` | Reset layout | `karya agent reset` |
| `Ctrl-a P` | New project (`lang:name`) | `karya new` |
| `Ctrl-a S` | Toggle synchronize-panes | tmux native |
| `Ctrl-a g` | Git window (lazygit) | tmux native |
| `Ctrl-a Q` | Kill session (confirm) | `karya dev -q` |

---

## 5. Repository / package structure

```
karya/
├── go.mod                       # module github.com/drjzlyan/karya
├── Makefile                     # build, test, lint, release helpers
├── main.go                      # thin entrypoint → internal/cli
├── cmd/karya/                   # (reserved) alt entrypoint if needed
├── internal/
│   ├── cli/                     # command dispatch + flag parsing
│   ├── config/                  # XDG paths, karya prefix, asset extraction
│   ├── version/                 # version string + build metadata
│   ├── tmuxx/                   # tmux command wrapper (isolated socket)
│   ├── session/                 # `dev`: build layout, panes, git window, quit
│   ├── agent/                   # detection, switch/next/prev/reset, prefs
│   ├── editor/                  # nvim launch (NVIM_APPNAME), `edit` routing
│   ├── project/                 # `new`: per-language scaffolds
│   ├── lang/                    # language selection + isolated mise generation
│   ├── tools/                   # tool detection + install into karya prefix
│   ├── prefs/                   # per-project key=value preference store
│   ├── doctor/                  # health checks
│   └── update/                  # self-update (GitHub releases + checksum)
├── assets/                      # go:embed sources (single-binary payload)
│   ├── nvim/                    # vendored copy of nvim-config (Lua)
│   ├── tmux.conf                # karya tmux config
│   ├── starship.toml            # optional prompt (never overwrites user)
│   └── manifest.json            # asset version → drives update/extraction
├── docs/
│   ├── PLAN.md                  # this file
│   ├── isolation.md             # deep dive on the zero-impact model
│   ├── commands.md              # per-command reference
│   └── languages.md             # per-language tooling
├── ROADMAP.md                   # phased milestones
├── PROGRESS.md                  # living status log (resume point)
├── AGENT.md                     # instructions for the agent/dev resuming work
└── README.md                    # user-facing intro + quick start
```

### Dependency policy

- **Standard library first.** Phase 0 uses only stdlib (`flag`, `os/exec`,
  `embed`) so the first build needs no network.
- Add [`spf13/cobra`](https://github.com/spf13/cobra) for the CLI once the
  command tree grows (Phase 1+). Keep the dependency list minimal.
- Self-update and release via [`goreleaser`](https://goreleaser.com) +
  `go:embed`. No CGO — pure-Go static binaries, trivial cross-compile.

---

## 6. Key subsystem designs

### 6.1 tmux orchestration (`internal/tmuxx`, `internal/session`)

- All tmux invocations go through `-L karya` (dedicated socket) and, on server
  start, `-f <extracted tmux.conf>`. This isolates karya's server from the
  user's tmux entirely.
- `session.Dev` reproduces the current `dev.sh` layout:
  editor (65%) | agent (top-right) / build+test (bottom-right), plus a `git`
  window running lazygit. Pane IDs + agent state stored in tmux session options
  (`@ide_*`), same scheme as today so behavior is identical.
- `NVIM_APPNAME=karya`, `EDITOR=karya edit`, `VISUAL=karya edit`,
  `GIT_EDITOR=karya edit` are set in the session environment so all
  editor-opening actions route into the editor pane.

### 6.2 Agent management (`internal/agent`)

- Detect known agents on `PATH` in preference order:
  `crush, claude, codex, gemini, aider, copilot`.
- Resolution order for the active agent: `--agent` flag → saved per-project
  preference → single detected agent → interactive picker.
- Per-project preference persisted to `~/.local/share/karya/prefs` (via
  `internal/prefs`), keyed by absolute workdir. Mirrors today's
  `ide-preferences.local` semantics.
- Switch/next/prev respawn the agent pane; reset rebuilds the layout while
  preserving the editor pane — a faithful port of `ide-agent.sh`.

### 6.3 Editor integration (`internal/editor`)

- `karya edit <file> [line]` finds the editor pane in the current karya tmux
  session and sends `:e +<line> <file>`; falls back to launching nvim directly
  when not in a session. Port of `bin/nvim-edit`.
- The Neovim config is the vendored `nvim-config`, embedded and extracted to
  `~/.config/karya/nvim`. Launched as `NVIM_APPNAME=karya nvim`. Plugin sync via
  `nvim --headless +"Lazy! sync" +qa` against the isolated data dir.
- `nvim-config`'s existing `:DevHealth`, `:DevUpdate`, etc. remain available and
  are surfaced through `karya doctor` / `karya update`.

### 6.4 Language & tool management (`internal/lang`, `internal/tools`)

- `karya lang` selects languages + runtime versions (queried from
  `mise ls-remote`), writes selection to
  `~/.local/share/karya/languages.local`, and generates an **isolated** mise
  config under the karya prefix — never the user's global mise.
- `internal/tools` installs LSP servers / formatters / debug adapters into
  `~/.local/share/karya/tools/`, detecting existing system installs first.
  Always-on: lua_ls, jsonls, yamlls, bashls, taplo, marksman. Selectable per
  language: basedpyright+ruff, jdtls+google-java-format, ts-language-server,
  gopls, clangd, rust-analyzer.

### 6.5 Self-update (`internal/update`)

- `karya update --check` queries the GitHub releases API for the latest tag.
- `karya update` downloads the platform asset + checksum, verifies, and atomically
  replaces the running binary (write to temp → rename). It also re-extracts
  embedded configs when the asset `manifest.json` version changed, refreshes
  managed tools, and runs `Lazy! sync`.
- Version metadata injected at build time via `-ldflags` into `internal/version`.

---

## 7. Distribution

- **GitHub Releases** with `goreleaser`: darwin/linux × amd64/arm64 static
  binaries + `SHA256SUMS`.
- **Install script**: `curl -fsSL <url>/install.sh | sh` downloads the right
  binary to `~/.local/bin/karya` and runs `karya install`.
- **Homebrew tap** (later): `brew install drjzlyan/tap/karya`.
- Self-update path for everyone via `karya update`.

---

## 8. Open questions / deferred decisions

- **Native agent** (own LLM API loop) is explicitly deferred; the agent
  interface is designed to allow plugging one in later (see ROADMAP Phase 8).
- **Linux tool bootstrap** parity (no Homebrew) — designed for, validated later.
- **Ghostty / terminal config** stays optional and never overwrites user files;
  may ship as a documented sample rather than an applied config.

See [ROADMAP.md](../ROADMAP.md) for the phased build order and
[PROGRESS.md](../PROGRESS.md) for the current resume point.
