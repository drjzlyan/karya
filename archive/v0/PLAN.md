# karya — Architecture & Design Plan

> **karya** (कार्य — "work/task") is an AI-first, terminal-based IDE delivered as a
> single self-contained Go binary. It installs, launches, manages, and updates a
> complete terminal development environment — a Neovim editor, a tmux multiplexer,
> and a coding agent, all orchestrated as one program — **without touching any of
> the user's existing settings.**

---

## 1. Product vision

Today the IDE is assembled from many moving parts:

- **Neovim** with a capability-oriented Lua config (LSP, completion, DAP, git,
  tasks, testing, 6 languages, health checks).
- **tmux** driving a 3-pane layout (editor / agent / build+test) plus a git window.
- **Session tooling** for the IDE layout, agent control, command routing, project
  scaffolding, language selection, and install/update/health.
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

A traditional config-management approach **symlinks over** `~/.zshrc`,
`~/.tmux.conf`, `~/.config/nvim`, etc. That is exactly what karya must **not** do.
karya achieves "installation does not impact any other user settings" through
strict namespacing:

| Concern | User's setup (untouched) | karya's isolated equivalent |
|---|---|---|
| Binary | — | `~/.local/bin/karya` (single file) |
| Config | `~/.config/nvim`, `~/.tmux.conf` | `~/.config/karya/**` |
| Data | `~/.local/share/nvim` | `~/.local/share/karya/**` |
| State | `~/.local/state/nvim` | `~/.local/state/karya/**` |
| Cache | `~/.cache/nvim` | `~/.cache/karya/**` |
| Neovim | user's `nvim` config | launched with `NVIM_APPNAME=karya/nvim` |
| tmux | user's default socket + `~/.tmux.conf` | dedicated socket `-L karya` + `-f` karya conf |
| Shell | user's `~/.zshrc` | **opt-in** `eval "$(karya shellenv)"` only |
| Tools | user's Homebrew | karya prefix `~/.local/share/karya/tools/bin` (detect-or-install) |

Key mechanisms:

- **`NVIM_APPNAME=karya/nvim`** — Neovim natively reads config from
  `~/.config/karya/nvim` and data from `~/.local/share/karya/nvim` when this env
  var is set (path separators nest it below the karya prefix). The user's own
  `~/.config/nvim` is never read or written. This is the clean, first-class
  isolation primitive Neovim provides.
- **Dedicated tmux server** — `tmux -L karya -f <karya tmux.conf>` runs on a
  separate socket with its own config. karya's sessions never collide with the
  user's tmux, and the user's `~/.tmux.conf` is never sourced.
- **Opt-in shell integration** — karya works fully as a bare binary on `PATH`.
  Users who want aliases / `EDITOR` routing / PATH additions add a single line:
  `eval "$(karya shellenv)"`. karya never rewrites shell rc files itself.
- **Tool management** — karya prefers tools already on `PATH`; anything it needs
  and cannot find, it installs into its own prefix (never into Homebrew or the
  user's global env). This extends to the **core runtime**: a fresh machine with
  only the karya binary gets tmux, Neovim, and the language toolchains installed
  via a karya-vendored, isolated mise (see §6.4). Homebrew is optional, not
  required.

**Uninstall guarantee:** `karya uninstall` deletes the karya prefix directories
and the binary, and nothing else. No user config is mutated at any point, so
there is nothing to restore.

---

## 3. Command & subsystem map

Every capability karya provides maps to a command or subsystem:

| Capability | karya surface |
|---|---|
| tmux IDE session | `karya` / `karya dev [name] [path]` |
| switch/next/prev/reset/status/prefs for the agent | `karya agent <subcommand>` |
| route a command to the build/test pane | `karya run <cmd>` |
| open a file in the editor pane | `karya edit <file> [line]` (also the `$EDITOR`) |
| scaffold python/java/ts/go/cpp/rust | `karya new <lang> <name> [dir]` |
| select langs + versions, generate isolated mise config | `karya lang [list\|add\|remove\|all]` |
| isolated, non-destructive install | `karya install` |
| upgrade karya + refresh configs/tools/plugins | `karya update` |
| no symlinking of user config | embedded configs + `NVIM_APPNAME` (never touches `~/.config/nvim`) |
| health checks | `karya doctor` (+ in-editor `:DevHealth`) |
| Lua editor config | embedded asset, extracted to `~/.config/karya/nvim` |
| tmux config | embedded asset, karya-owned socket + conf |
| shell aliases / PATH | `karya shellenv` (opt-in) |
| starship / lazygit / ghostty / gitconfig | embedded/optional; never overwrite user files |
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
karya agent focus             Jump to the agent pane
karya agent send [flags]      Paste stdin (+ --file/--line/--label header) into the agent pane
karya agent native [prompt]   Run karya's built-in Claude-API agent (needs ANTHROPIC_API_KEY)
karya agent prefs | clear     Inspect / clear per-project preference

karya edit <file> [line]      Open a file in the editor pane (used as $EDITOR)
karya run [-d dir] <cmd...>   Run a command in the build/test pane
karya run --focus             Focus the build/test pane

karya new <lang> <name> [dir] Scaffold a project (python|java|typescript|go|cpp|rust)
karya ship [--push --pr]      Stage, agent-write the commit message, commit (--no-verify)

karya task new "<prompt>"     Create a task: isolated worktree (branch karya/<id>) + agent
  --agent <name|none>         Agent for this task (else per-project pref / picker)
  --plan                      Draft a plan and hold at awaiting-plan for approval
karya task list | tasks       List the project's tasks + status
karya task dashboard          Fleet view: pick a task to switch to (Ctrl-a T)
karya task switch <id>        Attach to a task's session (rooted at its worktree)
karya task plan <id>          Show the drafted plan
karya task approve-plan <id>  Approve the plan: awaiting-plan → working
karya task review [<id>]      Show the task's diff vs its base (pre-apply review)
karya task merge [<id>] [--push]  Commit + merge karya/<id> into the project branch
karya task reject [<id>]      Mark the task rejected (worktree kept)
karya task checkpoint [<id>] [label]  Record a restorable snapshot
karya task rewind [<id>] [index|sha]  Reset the worktree to a checkpoint
karya task allow <action>     Pre-authorize merge|push|rewind for this project
karya task rm <id> [-y]       Remove a task: its worktree, branch, and record
  (id defaults to the current task inside a task-<id> session)

karya lang                    Interactive language + version selector
karya lang list | add | remove | all

karya install                 Set up karya (isolated): extract configs, fetch tools
karya update [--check]        Self-update binary + configs + tools + editor plugins
karya uninstall               Remove karya entirely (nothing else touched)

karya doctor                  Health check (tools, versions, isolation, LSPs)
karya shellenv                Print opt-in shell integration (eval this)
karya version                 Print version / build info
karya completion <shell>      Shell completions
karya keys                    Show the full CLI / tmux / Neovim key reference
```

Neovim leader scheme (Phase 8): one context-aware `<leader>c` **"Code"** group is
identical in every language — `<leader>ct` nearest test, `<leader>cf` format,
`<leader>cc` build, `<leader>cp` run project, `<leader>cR` run file, `<leader>cr`
refactor, `<leader>ci` organize imports, `<leader>cd` debug — dispatched to the
active buffer's language (no per-language `<leader>p/o/j/r/C/y` prefixes). A
`<leader>a` **"Agent"** group bridges editor→agent (`ab` buffer, `as` selection,
`ad` diagnostic, `af` file ref, `ac` explain, `aa` focus). A `<leader>k` **"Karya
Tasks"** group (`features/karyatasks.lua`) drives the task gates from the editor —
`kn` new, `kl` list, `kr` review, `km` merge, `kj` reject, `kc` checkpoint, `kw`
rewind — defaulting to the current task session and running in the build pane.
Close-buffer is `<leader>x`. `util/langmaps.lua` is the single registration point
for the `<leader>c` interface; an integration guardrail enforces the cross-language
interface and that the `<leader>k` task group stays bound.

In-session tmux keybindings (embedded `tmux.conf`, prefix `Ctrl-a`), preserved
from the current setup and wired to karya:

| Key | Action | Backed by |
|---|---|---|
| `Ctrl-a A` | Switch agent (prompt) | `karya agent switch` |
| `Ctrl-a N` | Next agent | `karya agent next` |
| `Ctrl-a D` | Reset layout | `karya agent reset` |
| `Ctrl-a P` | New project (`lang:name`) | `karya new` |
| `Ctrl-a G` | Ship (agent commit & push) | `karya ship --push` |
| `Ctrl-a T` | Tasks dashboard (fleet view) | `karya task dashboard` |
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
│   ├── agent/                   # detection, switch/next/prev/reset/focus/send, prefs, headless
│   ├── editor/                  # nvim launch (NVIM_APPNAME), `edit` routing
│   ├── ship/                     # `ship`: deterministic git + agent-authored commit message
│   ├── project/                 # `new`: per-language scaffolds
│   ├── lang/                    # language selection + isolated mise generation
│   ├── tools/                   # tool detection + install into karya prefix
│   ├── prefs/                   # per-project key=value preference store
│   ├── doctor/                  # health checks
│   ├── update/                  # self-update (GitHub releases + checksum)
│   └── assets/                  # go:embed payload + extraction/versioning
│       ├── nvim/                # vendored Neovim config (Lua)
│       ├── tmux.conf            # karya tmux config
│       └── *.go                 # embed + extract + manifest/version logic
├── docs/                        # USER-FACING product docs (embedded in binary)
│   ├── tutorial.md              # self-guided tutorial
│   ├── keymaps.md               # CLI / tmux / nvim key reference
│   ├── isolation.md             # deep dive on the zero-impact model
│   ├── commands.md              # per-command reference
│   └── languages.md             # per-language tooling
├── PLAN.md                      # this file — architecture & design (internal)
├── ROADMAP.md                   # phased milestones (internal)
├── PROGRESS.md                  # living status log / resume point (internal)
├── AGENT.md                     # engineering guide for contributors (internal)
└── README.md                    # user-facing intro + quick start
```

Documentation is split by audience: **`docs/`** holds user-facing product docs
(tutorial, keymaps, command/language reference) — these are embedded in the
binary (Phase 7). The **internal** engineering docs (`PLAN.md`, `ROADMAP.md`,
`PROGRESS.md`, `AGENT.md`) live at the repo root and are never shipped to users.

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
- `session.Dev` builds the IDE layout:
  editor (65%) | agent (top-right) / build+test (bottom-right), plus a `git`
  window running lazygit. Pane IDs + agent state stored in tmux session options
  (`@ide_*`), so behavior is stable across sessions.
- `NVIM_APPNAME=karya/nvim`, `EDITOR=karya edit`, `VISUAL=karya edit`,
  `GIT_EDITOR=karya edit` are set in the session environment so all
  editor-opening actions route into the editor pane.

### 6.2 Agent management (`internal/agent`)

- Detect known agents on `PATH` in preference order:
  `crush, claude, codex, gemini, aider, copilot`.
- Resolution order for the active agent: `--agent` flag → saved per-project
  preference → single detected agent → interactive picker.
- Per-project preference persisted to `~/.local/share/karya/prefs` (via
  `internal/prefs`), keyed by absolute workdir.
- Switch/next/prev respawn the agent pane; reset rebuilds the layout while
  preserving the editor pane.
- **Pluggable engine (`agent.Runner`, Phase 9).** Every agent is driven through a
  small consumer-defined interface — `Name`, `InteractiveCommand` (the pane-launch
  command), and `Headless(ctx, dir, prompt)` (a one-shot invocation returning
  stdout, or `ErrNoHeadless`). `cliRunner` wraps each BYO-CLI; `nativeRunner`
  (Phase 13) is the second implementation. `agent.NewRunner` is the single
  factory; `session.Build`, `Manager.launch`, and `ship`/task authoring all go
  through it, so BYO-CLI vs native is transparent to every consumer.
- **Native engine (`internal/native`, Phase 13).** karya's own Claude-API tool-use
  loop (`read_file`/`write_file`/`run_command`) over stdlib `net/http` — no SDK
  dependency, default model `claude-opus-5`. Its reason to exist is **per-tool-call
  permission prompts**: each write or command the model requests passes through a
  human-answered `Permit` gate (reads are never gated; access is confined to the
  workspace) — the one thing karya cannot enforce for a BYO-CLI, closing the
  Phase 11 caveat. Offered only when `ANTHROPIC_API_KEY` is set; BYO-CLI is the
  default. Interactive form: `karya agent native`.

### 6.3 Editor integration (`internal/editor`)

- `karya edit <file> [line]` finds the editor pane in the current karya tmux
  session and sends `:e +<line> <file>`; falls back to launching nvim directly
  when not in a session.
- The Neovim config is the vendored editor config, embedded via `go:embed` and
  extracted to `~/.config/karya/nvim`, versioned by a `manifest.json` so `update`
  re-extracts only when the embedded tree changed. Launched as
  `NVIM_APPNAME=karya/nvim nvim`: the trailing `/nvim` makes Neovim read that
  extracted config and nest its data/state/cache under `…/karya/nvim`, one level
  below karya's own tmux.conf/prefs. Plugin sync via
  `nvim --headless +"Lazy! sync" +qa` against the isolated data dir.
- The embedded config's `:DevHealth`, `:DevUpdate`, etc. remain available and
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

#### Self-contained runtime bootstrap (`internal/tools/{mise,bootstrap}.go`)

karya is self-contained **in operation**, not just packaging: on a fresh
machine the single binary installs every runtime dependency itself, into the
karya prefix — never Homebrew or the user's global mise.

- `EnsureMise` downloads the platform mise release from GitHub, **SHA-256
  verified** against the release `SHASUMS256.txt`, into `tools/bin/mise`. No-op
  when mise already resolves. (Checksum filenames carry a `./` prefix, stripped
  by `parseShasums`.)
- `EnsureCore` (tmux + neovim) and `EnsureToolchains` (+ node/go/rust/uv) run
  `mise use --global <tool>@latest` against the **isolated** `MISE_*` env, so
  everything lands under `MiseData`/`MiseShims`. Detect-first, best-effort.
- Wiring: `karya`/`karya dev`/`karya new` auto-install missing **core** deps
  then continue (`ensureRuntime`); `karya install` runs the full
  `EnsureToolchains`; `karya lang` provisions mise on demand.
- **Tool resolution rule:** `config.Paths.ActivateManagedEnv()` prepends
  `ToolsBin` + `MiseShims` to karya's *own* process PATH **and** exports the
  `MISE_*` vars so shim-backed tools both resolve and run (PATH alone yields
  "not a valid shim"). Called in `newApp` and `cmdDoctor`; child processes get
  the same via `Paths.Env`.

### 6.5 Self-update (`internal/update`)

- `karya update --check` queries the GitHub releases API for the latest tag.
- `karya update` downloads the platform asset + checksum, verifies, and atomically
  replaces the running binary (write to temp → rename). It also re-extracts
  embedded configs when the asset `manifest.json` version changed, refreshes
  managed tools, and runs `Lazy! sync`.
- Version metadata injected at build time via `-ldflags` into `internal/version`.

### 6.6 Tasks & task-level isolation (`internal/task`, `internal/worktree`)

The human-in-the-loop, agents-first arc (ROADMAP Phases 10–13) makes the **task**
karya's primary noun and adds a second kind of isolation on top of the
environment isolation of §2: **task-level isolation**.

- A `task.Task` (id, title, prompt, agent, status, branch, worktree, repo,
  timestamps) is one unit of agent work. Its lifecycle is
  `planning → awaiting-plan → working → awaiting-review → merged | rejected`
  (the plan states are entered only when plan approval is requested — Phase 11).
- `task.Store` persists the tasks of one project as a single JSON file under
  `config.Paths.TasksDir()`, named by `worktree.ProjectSlug(repo)` so a project's
  tasks stay grouped and never touch the user's config.
- `worktree.Manager` gives each task an isolated `git worktree`: a namespaced
  branch `karya/<id>` checked out under `config.Paths.WorktreesDir()`
  (`~/.local/state/karya/worktrees/<project-slug>/<id>`) — **never** in the user's
  own tree. It runs git behind a consumer-defined `Runner` (the same shape as
  `ship.Runner`, satisfied by `ship.ExecRunner`), keeping the plumbing
  unit-testable. `Remove` force-removes the worktree, deletes the branch, prunes,
  and clears any residual dir, so a task leaves nothing behind.
- `karya task new/list/switch/rm` (`internal/cli/task.go`) drive it: `new` creates
  the worktree + record and, inside a karya session, opens a session rooted at the
  worktree with the task's agent; `switch` attaches; `rm` tears the task down. The
  agent thus works **inside the isolated worktree**, and the human reviews before
  merge.

**The four human-in-the-loop gates (Phase 11)** are layered on the task model,
all reusing the extended `ship.Git` plumbing (`RevParse`, `CommitAll`,
`DiffCachedAgainst`, `Merge`, `ResetHard`):

1. **Plan approval** — `task new --plan` drafts a plan through the agent's
   headless `Runner` and holds the task at `awaiting-plan`; `task approve-plan`
   advances it to `working` (validated by `task.CanTransition`).
2. **Diff review before apply** — `task review` shows the whole task diff against
   its recorded `BaseCommit` (the user's branch is still untouched, since the work
   lives on `karya/<id>`); `task merge` merges it in, `task reject` discards it.
3. **Checkpoint & rollback** — `task checkpoint` records a restorable commit;
   `task rewind` resets the worktree to one.
4. **Permission prompts** — `gateAction` confirms karya-initiated merge/push/
   rewind, honored by a per-project allowlist (`task allow`). It gates only
   karya's **own** actions — a BYO-CLI agent's internal tool calls are outside
   karya's reach until the native engine (Phase 13).

Gate commands default to the **current task** when run inside its `task-<id>`
session, so the `<leader>k` "Karya Tasks" editor group can invoke them id-free.

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

- **Human-in-the-loop, agents-first arc (Phases 9–13, complete).** karya evolved
  from "an agent in a pane" into a human-in-the-loop, AI-agents-first IDE built on
  a new **task-level** isolation (a worktree/branch per task) layered on the
  existing environment isolation. The pluggable `agent.Runner` interface (Phase 9)
  made the engine swappable; the **native agent** (own Claude-API tool-use loop,
  `internal/native`) landed as the second implementation behind it (Phase 13),
  unlocking per-tool-call permission prompts — **BYO-CLI stays the default**. See
  ROADMAP Phases 9–13 and §6.2/§6.6.
- **Linux tool bootstrap** parity (no Homebrew) — designed for, validated later.
- **Ghostty / terminal config** stays optional and never overwrites user files;
  may ship as a documented sample rather than an applied config.
- **Product, not a port.** karya stands on its own terms. It deliberately rejects
  the symlink-over-user-config model (see §2) and any other design that would
  break the isolation guarantee. The embedded editor tree
  (`internal/assets/nvim/`) is the sole source of truth — a clean checkout builds
  with no sibling repositories. Historical provenance lives in git history, not
  the tree.

See [ROADMAP.md](ROADMAP.md) for the phased build order and
[PROGRESS.md](PROGRESS.md) for the current resume point.
