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
| `karya agent prefs` / `clear` | Show / clear per-project agent preference |
| `karya edit <file> [line]` | Open a file in the editor pane (used as `$EDITOR`) |
| `karya run [-d dir] <cmd>` | Run a command in the build/test pane |
| `karya run --focus` | Focus the build/test pane |
| `karya new <lang> <name>` | Scaffold a project (python/java/typescript/go/cpp/rust) |
| `karya lang` | Select languages and runtime versions |
| `karya install` | Set up karya (isolated, non-destructive) |
| `karya update` | Self-update binary, configs, tools, editor plugins |
| `karya uninstall` | Remove karya entirely (nothing else touched) |
| `karya doctor` | Health check |
| `karya shellenv` | Print opt-in shell integration (`eval "$(karya shellenv)"`) |
| `karya version` | Version / build info |

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
| `Ctrl-a S` | Toggle synchronize-panes |
| `Ctrl-a g` | Open lazygit (reuse or create the `git` window) |
| `Ctrl-a Q` | Kill the IDE session (confirm) |

---

## 3. Neovim (leader `Space`)

### Global

| Key | Mode | Action |
|---|---|---|
| `<Esc>` | n | Clear search highlight |
| `<leader>S` | n | Save |
| `<leader>Z` | n | Quit |
| `<leader>c` | n | Close buffer |
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

### Testing (generic) & tasks

| Key | Action |
|---|---|
| `<leader>Tt` / `<leader>Tc` | Run nearest test / test class |
| `<leader>Tp` / `<leader>Tm` | Run package / module tests |
| `<leader>Tl` | Re-run last test |
| `<leader>Td` / `<leader>TD` | Debug nearest / test class |
| `<leader>mb` / `<leader>ms` | Build / test task |
| `<leader>mc` / `<leader>mp` | Clean / run project task |
| `<leader>t` | Focus tmux build/test pane |

### Git

| Key | Mode | Action |
|---|---|---|
| `<leader>gd` | n | Diffview (current changes) |
| `<leader>gh` | n | Preview hunk |
| `<leader>gb` | n | Blame line |
| `<leader>gs` / `<leader>gr` | n/v | Stage / reset hunk |
| `<leader>gu` | n | Undo stage hunk |
| `<leader>gn` / `<leader>gp` | n | Next / previous hunk |
| `<leader>gD` | n | Diff against index |

(Commit/push/branch live in lazygit: `Ctrl-a g`.)

### Per-language prefixes

Every language shares the same suffixes under its own leader prefix, registered
buffer-locally: `f` format · `i` organize imports · `r` refactor · `c` build ·
`p` run project · `R` run file · `t`/`T` test nearest/class · `d`/`D` debug ·
`h`/`H` call hierarchy.

| Language | Prefix | Example |
|---|---|---|
| Python | `<leader>p` | `<leader>pt` pytest file, `<leader>ptf` test under cursor |
| Java | `<leader>j` | `<leader>jc` compile, `<leader>jt` nearest test |
| TypeScript/JS | `<leader>y` | `<leader>yt` nearest `node:test`, `<leader>yi` imports |
| Go | `<leader>o` | `<leader>ot` nearest test, `<leader>od` debug, `<leader>lI` imports |
| C/C++ | `<leader>C` | test maps run the `ctest` suite in `build/` |
| Rust | `<leader>r` | `<leader>rt` nearest `#[test]` |

Java also has workspace commands under `<leader>W` (build/reload/restart
jdtls/clear cache/logs). Each action also exists as a command, e.g.
`:GoTestNearest`, `:RustRunFile`, `:CppBuild`.

### Developer commands (from the embedded config)

`:DevHealth` · `:DevInfo` · `:DevReload` · `:DevUpdate` · `:DevProfile` ·
`:DevCleanCache [treesitter|jdtls|swap|sessions|lazy|all]`

### Discoverability

Press `<leader>` and pause — `which-key` shows every group. You rarely need to
memorize anything.
