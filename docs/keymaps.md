# karya keymaps & commands reference

karya is **one program with one keymap**. There is a single leader —
**`Ctrl-Space`** (written `<L>` below) — and the *same* chords drive everything:
moving between panes, resizing, splitting, switching tabs, git, tasks, and gates.
Whatever holds focus — the editor, a shell, or an agent — the leader works the
same way. Keys karya does not claim are forwarded to the focused pane (into the
embedded editor, or a shell). For a guided path, see [tutorial.md](tutorial.md);
for the full command list, [commands.md](commands.md).

> **One leader, one grammar.** Earlier karya layered three separate keymaps — a
> tmux prefix, a Neovim leader, and lazygit's bindings. That is gone. karya now
> draws its own UI and routes every keystroke through a single keymap engine, so
> there is nothing per-tool to memorize.

> This reference describes the single-process TUI. Run `karya doctor` to see
> what your installed build provides.

> **Changing the leader.** `Ctrl-Space` is the default, but some environments
> intercept it — most notably **macOS**, where *System Settings → Keyboard →
> Keyboard Shortcuts → Input Sources* binds `Ctrl-Space` to "Select the previous
> input source", so it never reaches the terminal. If the leader does nothing,
> either turn that macOS shortcut off, or pick another leader with the
> `KARYA_LEADER` environment variable, e.g. `KARYA_LEADER=ctrl+a karya tui`.
> Accepted values: `ctrl+space` (default) or `ctrl+<letter>` (e.g. `ctrl+a`,
> `ctrl+b`, `ctrl+g`). Whatever you choose, it is the single leader for
> everything below, and the status line shows it.

---

## Discoverability: which-key

Press **`Ctrl-Space`** and pause. A popup shows every continuation from here —
the groups (`t` tasks, `g` git, …) and the leaf actions. Keep typing to drill in;
`Esc` cancels. You rarely need to memorize anything: the leader *is* the menu.

The current mode (Passthrough / Leader / Command / Search) is always shown in the
status line, so you are never trapped.

---

## 1. Window, pane & tab control (`<L>` = `Ctrl-Space`)

The one set of bindings for the whole IDE — no separate multiplexer.

| Key | Action |
|---|---|
| `<L> h` / `j` / `k` / `l` | Focus the pane to the left / down / up / right |
| `<L> H` / `J` / `K` / `L` | Resize the focused pane (repeatable) |
| `<L> \|` | Split the focused pane to the right |
| `<L> -` | Split the focused pane downward |
| `<L> =` | Equalize all splits |
| `<L> z` | Zoom / unzoom the focused pane (toggle fullscreen) |
| `<L> x` | Close the focused pane (confirm) |
| `<L> w` | Pane / window switcher (pick from a list) |
| `<L> c` | New tab |
| `<L> n` / `<L> p` | Next / previous tab |
| `<L> 1`–`9` | Jump to tab by number |
| `<L> e` | Focus the editor pane |
| `<L> f` | Find file (fuzzy) — opens the selection in the editor |
| `<L> /` | Search the project (live grep) — opens a match at its line |
| `<L> b` | Focus the build/test pane; run the last command |
| `<L> ?` | Full keymap & command reference |
| `<L> Q` | Quit karya (confirm) |
| `Ctrl-Space Ctrl-Space` | Send a literal `Ctrl-Space` to the focused pane |

## 2. Tasks — `<L> t` (human-in-the-loop)

Each task is a spec contract (`.karya/tasks/<id>/SPEC.md`) that runs in its own
isolated git worktree (branch `task/<id>`) and advances through human gates.

| Key | Action |
|---|---|
| `<L> t t` | Task board (every task, its state, agent, title) |
| `<L> t n` | New task (prompts for a slug; scaffolds the spec) |
| `<L> t s` | Start a task (create its isolated worktree) |
| `<L> r` | Review the pending gate for the current task |
| `<L> a` | Agent inbox / delegate the current gate to an agent |

On the task board the whole gate lifecycle is keyboard-driven — each key runs the
matching `karya` command in the background and updates the row's state in place:

| Key | Action |
|---|---|
| `j`/`k` | Move the selection |
| `n` | New task (type a slug, `Enter` creates it and opens its spec) |
| `s` | Start the selected task (create its isolated worktree) |
| `p` | Plan — the agent drafts `PLAN.md` (→ plan gate) |
| `i` | Implement the approved plan (→ diff gate) |
| `v` | Verify — run the spec's Verification block (→ verify gate) |
| `m` | Merge the approved worktree back to base |
| `Enter` | Review the selected task's pending gate |
| `a` | Run the task's agent interactively in a pane |
| `r` | Refresh · `q` closes |

Review and approval stay native (`Enter` / `<L> r`): a human always crosses the
gate. Destructive actions (abandon) confirm.

## 3. Git — `<L> g` (built-in panel)

karya has its own git panel — no external git TUI.

| Key | Action |
|---|---|
| `<L> g g` | Open the git panel (status, stage, diff, log, branches) |
| `<L> g c` | Commit (agent can draft the message) |
| `<L> g p` | Push |

The panel is a set of bordered panes — **Changes** (working tree), **Branches**,
**Stashes**, and **Log** (recent history) stacked on the left, with a live diff of
the selected item on the right — so it stays useful even on a clean tree and it's
always clear which section has focus.

`Tab` (and `Shift-Tab`) cycles focus between the panes; `j`/`k` move within the
focused pane; `Enter` performs its primary action for that pane. Per pane:

| Pane | `Enter` / keys |
|---|---|
| Changes | `Space`/`Enter` stage/unstage · `a`/`u` stage/unstage all · `c` commit · `P` push |
| Branches | `Enter` checks out the selected branch (current marked `*`) · `b` creates a new branch |
| Stashes | `Enter` pops the selected stash · `s` stashes the working tree |
| Log | moving previews each commit's diff |

`s` (stash) and `b` (new branch) work from any pane. `Ctrl-d`/`Ctrl-u` scroll the
diff; `q` closes. The diff view is the same renderer used by task review.

## 4. Editing (the embedded Neovim engine)

The editor pane is Neovim embedded as an engine: full modal editing, LSP,
treesitter, and completion. Because karya forwards unclaimed keys straight to it,
**normal Vim editing works as you expect** — `hjkl`, operators, text objects,
`:` commands, visual mode, and so on. karya only intercepts the `Ctrl-Space`
leader (for IDE actions) before Neovim sees it.

Editor-local actions (format, code action, go-to-definition, run test, …) are
provided by karya's slim Neovim config under Neovim's own `Space` leader inside
the editor, and are also reachable as IDE actions from `Ctrl-Space` where they
cross panes (e.g. running tests routes output to the build/test pane). The full,
current editor action list is shown by `<L> ?`.

Common LSP keys (in the editor pane, under Neovim's own `Space` leader): `gd`
definition, `gD` declaration, `gr` references, `gi` implementation, `gy` type
definition, `K` hover, `<C-k>` signature help, `<Space>ds`/`<Space>ws` document/
workspace symbols, `<Space>e` show diagnostic, `[d`/`]d` prev/next diagnostic,
`<Space>q` diagnostics list, `<Space>rn` rename, `<Space>ca` code action,
`<Space>f` format. The servers are the ones karya auto-installs. For fuzzy file
navigation and project-wide search across the whole repo, use the karya views
`<L> f` and `<L> /` above (they open results back in the editor).

**Zero-setup language servers.** When you open a file, karya auto-installs that
language's server (and formatter/linter) into its own isolated prefix in the
background — no `:Mason`, no manual install. The status line shows
`installing go language tools…`, then the LSP attaches on its own when ready.
Nothing is installed globally; only the languages you actually open get tooling.
Supported today: Go, Python, Rust, TypeScript/JavaScript, C/C++ (plus the
always-on Lua/JSON/YAML/Bash/Markdown/TOML servers). More via the marketplace
later.

## 5. Modes

| Mode | How you enter it | What it does |
|---|---|---|
| Passthrough | default | Keys flow to the focused pane; only `<L>` (and a tiny always-on set) are intercepted |
| Leader | `Ctrl-Space` | Next keys resolve against the keymap; which-key popup appears |
| Command | `:` in a karya view | Type a karya view command |
| Search | `/` in a karya view/list | Incremental search |

`Esc` always returns to Passthrough.

---

For every `karya` subcommand (`task`, `plan`, `implement`, `review`, `gate`,
`verify`, `merge`, `lang`, `install`, `doctor`, …) see [commands.md](commands.md)
or run `karya help <command>`.
