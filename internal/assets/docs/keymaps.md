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

On the task board and in lists, vim motions work: `j`/`k` move, `g`/`G` jump,
`/` searches, `Enter` opens, `q` closes. Destructive actions (abandon) confirm.

## 3. Git — `<L> g` (built-in panel)

karya has its own git panel — no external git TUI.

| Key | Action |
|---|---|
| `<L> g g` | Open the git panel (status, stage, diff, log, branches) |
| `<L> g c` | Commit (agent can draft the message) |
| `<L> g p` | Push |

Inside the git panel: `j`/`k` move, `Space`/`Enter` stage/unstage the item under
the cursor, `c` commit, `P` push, `d` view the diff, `q` closes. The diff view is
the same renderer used by task review.

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

Common LSP editing keys (in the editor pane): `gd` definition, `gr` references,
`gi` implementation, `K` hover, `<Space>la` code action, `<Space>lr` rename,
`<Space>lf` format. Language actions (build/run/test) are consistent across every
supported language — the same keys work whatever the file is.

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
