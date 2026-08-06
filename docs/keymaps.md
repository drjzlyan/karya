# karya keymaps & commands reference

karya is three layers you drive together: the **`karya` CLI**, the **tmux** IDE
session (prefix `Ctrl-a`), and **Neovim** (leader `Space`). This is the complete
reference. For a guided, hands-on path through it, see
[tutorial.md](tutorial.md).

> Editor keymaps below are provided by karya's embedded Neovim config, which is
> extracted on `karya install`. CLI/tmux orchestration is available now; run
> `karya doctor` to see what's available in your installed build.

---

## 1. karya CLI

| Command | Action |
|---|---|
| `karya` | Launch or attach the IDE session for the current directory |
| `karya dev [name] [path]` | Explicit session launch (flags below) |
| `karya dev -a <agent>` | Choose the coding agent (`none` for a plain shell) |
| `karya dev -k` | Kill an existing session and recreate it |
| `karya dev -q` | Quit (kill) the session cleanly |
| `karya agent status` | Show current/available agents + saved preference |
| `karya agent switch` | Interactive agent switcher (in session) |
| `karya agent next` / `prev` | Cycle agents |
| `karya agent reset` | Reset pane layout (preserves the editor) |
| `karya agent focus` | Jump to the agent pane |
| `karya agent send [--file f --line n --label t]` | Paste stdin into the agent pane (editor↔agent bridge) |
| `karya agent native [prompt]` | Run karya's built-in Claude-API agent — approves each file write / command (needs `ANTHROPIC_API_KEY`) |
| `karya agent prefs` / `clear` | Show / clear per-project agent preference |
| `karya edit <file> [line]` | Open a file in the editor pane (used as `$EDITOR`) |
| `karya run [-d dir] <cmd>` | Run a command in the build/test pane |
| `karya run --focus` | Focus the build/test pane |
| `karya new <lang> <name>` | Scaffold a project (python/java/typescript/go/cpp/rust) |
| `karya ship [--push --pr --no-verify]` | Stage, let the agent write the commit message, commit (& push / open PR) |
| `karya init [--force]` | Scaffold `.karya/` + a repo `AGENTS.md` for the task workflow |
| `karya task new <slug> [--agent n]` | Scaffold a task spec (`.karya/tasks/<date>-<slug>/SPEC.md`) |
| `karya task list` / `tasks` | The task board (also `Ctrl-a T`) |
| `karya task status` | Per-state counts + the gate inbox |
| `karya task show <id>` | Task detail: state, workspace, spec summary, gate history |
| `karya task start <id> [--base <ref>]` | Create the task worktree (branch `task/<id>`) |
| `karya task abandon <id> [-y]` | Remove the task: worktree, branch, and artifacts |
| `karya lang` | Interactive language + runtime-version selector |
| `karya lang list` | Show the selected languages and versions |
| `karya lang add <lang> [versions]` | Add/change a language (installs its tooling) |
| `karya lang remove <lang>` | Drop a language from the selection |
| `karya lang all` | Select every language at its latest stable version |
| `karya install` | Set up karya (isolated, non-destructive) |
| `karya update` | Self-update binary, configs, tools, editor plugins |
| `karya uninstall` | Remove karya entirely (nothing else touched) |
| `karya doctor` | Health check: tools, versions, isolation & per-language tooling |
| `karya shellenv` | Print opt-in shell integration (`eval "$(karya shellenv)"`) |
| `karya completion <shell>` | Print a bash/zsh/fish completion script to source |
| `karya keys` | Show this full CLI / tmux / Neovim key reference |
| `karya version` | Version / build info |
| `karya tutorial [n]` | Run the self-working tutorial (verifies against a sandbox) |
| `karya docs [topic]` | Read the embedded docs offline (no topic lists them) |
| `karya help [command]` | General help, or detailed help for one command |

> Note: options come before positionals — `karya dev -a claude myproj`.

---

## 2. tmux (prefix `Ctrl-a`)

Bindings marked *(default)* are standard tmux built-ins.

### Sessions / windows / panes

| Key | Action |
|---|---|
| `Ctrl-a d` | Detach (session keeps running) |
| `Ctrl-a s` | Session switcher |
| `Ctrl-a c` | New window *(default)* |
| `Ctrl-a n` / `p` | Next / previous window *(default)* |
| `Ctrl-a 1`–`9` | Jump to window by number *(default)* |
| `Ctrl-a h/j/k/l` | Move to pane left/down/up/right |
| `Ctrl-a H/J/K/L` | Resize pane (repeatable) |
| `Ctrl-a \|` / `-` | Split right / below |
| `Ctrl-a z` | Zoom (toggle fullscreen) pane *(default)* |
| `Ctrl-a [` | Copy mode (scroll; `v` select, `y` yank, `q` exit) |
| `Ctrl-a r` | Reload karya tmux config |

### karya bindings (call the `karya` binary)

| Key | Action |
|---|---|
| `Ctrl-a A` | Switch agent (interactive) → `karya agent switch` |
| `Ctrl-a N` | Next agent → `karya agent next` |
| `Ctrl-a D` | Reset layout → `karya agent reset` |
| `Ctrl-a P` | New project (`language:name`) → `karya new` |
| `Ctrl-a G` | Ship: agent writes the commit message, then commit & push → `karya ship --push` |
| `Ctrl-a T` | Task board popup → `karya task list` |
| `Ctrl-a S` | Toggle synchronize-panes |
| `Ctrl-a g` | Open lazygit (reuse or create the `git` window) |
| `Ctrl-a ?` | Pop up this key map & command reference → `karya docs keymaps` |
| `Ctrl-a Q` | Kill the IDE session (confirm) |

---

## 3. Neovim (leader `Space`)

### Global

| Key | Mode | Action |
|---|---|---|
| `<Esc>` | n | Clear search highlight |
| `<leader>S` | n | Save |
| `<leader>Z` | n | Quit |
| `<leader>x` | n | Close buffer |
| `<leader>-` / `<leader>\|` | n | Split below / right |
| `<leader>=` | n | Equalize splits |
| `<C-h/j/k/l>` | n | Navigate windows |
| `<A-j>` / `<A-k>` | n/v | Move line(s) down / up |
| `<` / `>` | v | Indent (selection stays active) |

### Files, search, explorer

| Key | Action |
|---|---|
| `<leader>ff` | Find files |
| `<leader>fr` | Recent files |
| `<leader><space>` | Buffers |
| `<leader>s/` | Live grep |
| `<leader>s*` | Grep word under cursor |
| `<leader>st` | Search TODO/FIXME |
| `<leader>:` | Command history |
| `<leader>E` | Explorer at current file's dir (oil) |
| `<leader>O` | Explorer at working dir |

### LSP (shared across all languages)

| Key | Mode | Action |
|---|---|---|
| `gd` / `gD` | n | Definition / declaration |
| `gr` | n | References |
| `gi` / `gt` | n | Implementation / type definition |
| `K` | n | Hover docs |
| `<C-k>` | i | Signature help |
| `<leader>lr` | n | Rename symbol |
| `<leader>la` | n/v | Code action |
| `<leader>lf` | n/v | Format via LSP |
| `<leader>ls` / `<leader>ld` | n | Workspace / document symbols |

### Diagnostics (trouble)

| Key | Action |
|---|---|
| `<leader>ee` | Diagnostics list |
| `<leader>er` / `<leader>ei` | References / implementations |
| `<leader>en` / `<leader>ep` | Next / previous item |

### Editing: treesitter, comments, surround

| Key | Mode | Action |
|---|---|---|
| `gcc` / `gc` | n / n,v | Toggle line / motion comment |
| `sa` / `sd` / `sr` | n,v / n | Add / delete / replace surround |
| `gnn` `grn` `grm` `grc` | n | Init / grow node / grow scope / shrink |
| `af`/`if`, `ac`/`ic`, `ab`/`ib`, `ap`/`ip` | — | Around/inside function, class, block, parameter |
| `]f`/`[f`, `]c`/`[c`, `]b`/`[b`, `]p`/`[p` | n | Next/prev function, class, block, parameter |

### Debug (nvim-dap)

| Key | Action |
|---|---|
| `<leader>db` / `<leader>dB` | Toggle / conditional breakpoint |
| `<leader>dc` | Continue / start |
| `<leader>di` / `<leader>do` / `<leader>dO` | Step into / over / out |
| `<leader>dr` | REPL |
| `<leader>du` | Toggle DAP UI |
| `<leader>dt` / `<leader>dx` | Terminate / clear breakpoints |

### Code — `<leader>c` (identical in every language)

One context-aware group. The **same keys** work whatever the file is: karya
dispatches to the active buffer's language. No per-language prefixes to remember.

| Key | Mode | Action |
|---|---|---|
| `<leader>cf` | n/v | Format |
| `<leader>ci` | n | Organize imports *(languages with an organizer)* |
| `<leader>cr` | n/v | Refactor (code actions) |
| `<leader>cc` | n | Build / compile project |
| `<leader>cp` | n | Run project |
| `<leader>cR` | n | Run current file |
| `<leader>ct` / `<leader>cT` | n | Run nearest test / test file · class |
| `<leader>cl` | n | Re-run last test |
| `<leader>cd` / `<leader>cD` | n | Debug nearest test / test file *(debug-capable langs)* |
| `<leader>ch` / `<leader>cH` | n | Incoming / outgoing calls |
| `<leader>t` | n | Focus tmux build/test pane |

Language-specific extras stay under the same prefix, e.g. Python `<leader>cm`
(run module) / `<leader>cs` (run selection) / `<leader>cv` (show venv), and Java
`<leader>cw*` workspace actions (build/reload/restart jdtls/clear cache/logs/type
hierarchy/verify). Every action also exists as a command, e.g. `:GoTestNearest`,
`:RustRunFile`, `:CppBuild`.

### Agent — `<leader>a` (editor↔agent bridge)

Send editor context straight into the coding-agent pane — the agent feels part of
the editor, not a separate CLI. Text is pasted unsubmitted so you stay in control.

| Key | Mode | Action |
|---|---|---|
| `<leader>aa` | n | Focus the agent pane |
| `<leader>ab` | n | Send the whole buffer |
| `<leader>as` | v | Send the visual selection (with `file:line`) |
| `<leader>ac` | n/v | Explain code under cursor / selection |
| `<leader>ad` | n | Send the diagnostic on this line |
| `<leader>af` | n | Send a reference to the current file |

### Karya Tasks — `<leader>k` (human-in-the-loop)

Drive `karya task` from the editor. Each task is a spec contract
(`.karya/tasks/<id>/SPEC.md`) that runs in its own isolated git worktree
(branch `task/<id>`) and advances through human gates. These run in the
build/test pane so you see the board and gate history there.

| Key | Mode | Action |
|---|---|---|
| `<leader>kn` | n | New task (prompts for a slug; scaffolds the spec) |
| `<leader>kl` | n | List tasks (the task board) |
| `<leader>ks` | n | Show a task (state, spec summary, gate history) |
| `<leader>kt` | n | Start a task (create its isolated worktree) |
| `<leader>ka` | n | Abandon a task (remove worktree, branch, artifacts) |

### Git

| Key | Mode | Action |
|---|---|---|
| `<leader>gc` | n | **Ship**: stage, agent-write the commit message, commit → `karya ship` |
| `<leader>gd` | n | Diffview (current changes) |
| `<leader>gh` | n | Preview hunk |
| `<leader>gb` | n | Blame line |
| `<leader>gs` / `<leader>gr` | n/v | Stage / reset hunk |
| `<leader>gu` | n | Undo stage hunk |
| `<leader>gn` / `<leader>gp` | n | Next / previous hunk |
| `<leader>gD` | n | Diff against index |

(Manual commit/push/branch also live in lazygit: `Ctrl-a g`.)

### Developer commands (from the embedded config)

`:DevHealth` · `:DevInfo` · `:DevReload` · `:DevUpdate` · `:DevProfile` ·
`:DevCleanCache [treesitter|jdtls|swap|sessions|lazy|all]`

### Discoverability

Press `<leader>` and pause — `which-key` shows every group. You rarely need to
memorize anything.
