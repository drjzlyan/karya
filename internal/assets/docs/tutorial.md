# Tutorial: Learn the karya IDE

A hands-on, keystroke-by-keystroke walkthrough. Follow it top to bottom and you
will have installed karya, learned the core movements, managed coding agents, and
edited, built, tested, debugged, and committed code in Python, Java, TypeScript,
Go, C/C++, and Rust — all inside one terminal IDE.

> **Status:** karya is being rebuilt as a **single-process TUI IDE**: one program
> that draws its own screen (window/pane manager, git panel, task/review views)
> and embeds Neovim as the editing engine — all under **one leader key**,
> `Ctrl-Space`. This changes the IDE surface described later in this page. The
> authoritative, up-to-date key reference is [keymaps.md](keymaps.md); this
> walkthrough's per-language editor lessons still describe the target editor
> behavior and are being migrated to the new UI. The `karya task` workflow (specs,
> worktrees, gates) is stable. Run `karya version` and `karya doctor` to see
> what's available in your build.

Conventions:

- **`<L>` is the leader — `Ctrl-Space`.** It drives every IDE action (panes,
  tabs, git, tasks, gates), whatever pane has focus. There is no separate tmux
  prefix or Neovim leader for IDE actions anymore — one grammar (see
  [keymaps.md](keymaps.md)).
- Inside the editor pane, normal Vim editing works; Neovim keeps its own `Space`
  leader for editor-local text actions. Press `<Esc>` to return to Normal mode.
- `n` / `i` / `v` after a key means Normal / Insert / Visual mode.
- `:Foo` means type the command then press `Enter`.
- **Now try it** boxes mean *you* type the command or press the keys — that's how
  this walkthrough teaches. Don't just read them.

---

## Learn by doing: two interactive tutorials

This page is the reference walkthrough. karya also ships two **hands-on**
tutorials that ask *you* to type each step and verify it for real — start with
these, then use this page to go deeper.

- **The command tutorial** — a type-it-yourself tour of the CLI. Each lesson asks
  you to type a `karya` command; a throwaway sandbox then runs the real behavior
  and confirms it worked on your machine.

  > **Now try it — type:** `karya tutorial`
  >
  > (Add `list` to see the lessons, or a number to jump to one.)

- **The IDE tutorial** — a keystroke-by-keystroke walkthrough *inside* the real
  IDE. You pick a language, then practice the whole developer loop — nvim motions,
  files/search, LSP, the language-agnostic Code group, debugging, the agent
  bridge, tmux panes/windows, lazygit, and ship. karya detects each keystroke,
  runs the real operation, and advances automatically.

  > **Now try it — type:** `karya tutorial ide` (or `karya tutorial ide python`
  > to choose the language; without one it uses your primary selected language, else Go).
  >
  > Already inside a karya session? Just run `:KaryaTutorial` in the editor (or
  > press `<leader>?`). Stuck on a step? Jump into the tutorial panel with
  > `<C-w>w`, then press `s` to skip, `n` for next, or `q` to quit.

The rest of this page mirrors the IDE tutorial so you can read along or catch up
on anything you skipped.

---

## Part 0 — Fundamentals: tmux and Neovim

Skip this if you're already fluent in tmux and Neovim. Otherwise learn these
first — they recur in every later part.

### 0.1 tmux basics

tmux runs multiple shells in one window and keeps sessions alive after you close
the terminal. karya runs tmux on its **own dedicated server** (`tmux -L karya`),
so it never collides with any tmux you already use.

**Prefix key:** `Ctrl-a`. Every tmux command starts with the prefix.

| Concept | What it is |
|---|---|
| Session | A named group of windows. `karya` creates one per project. |
| Window | A tab inside a session (fills the terminal). |
| Pane | A split inside a window. The IDE layout uses 3 panes. |

Session management:

| Key / Command | Action |
|---|---|
| `Ctrl-a d` | Detach (session keeps running) |
| `Ctrl-a s` | Session switcher |
| `Ctrl-a Q` | Kill the IDE session (karya binding) |
| `karya dev -q` | Quit the session from the shell |

Window management:

| Key | Action |
|---|---|
| `Ctrl-a c` | New window *(default)* |
| `Ctrl-a n` / `p` | Next / previous window *(default)* |
| `Ctrl-a 1`–`9` | Jump to window by number *(default)* |
| `Ctrl-a g` | Open lazygit (reuse or create the `git` window) |
| `Ctrl-a P` | New project (prompts `language:name` → `karya new`) |

Pane navigation and management:

| Key | Action |
|---|---|
| `Ctrl-a h/j/k/l` | Move left/down/up/right |
| `Ctrl-a \|` / `-` | Split right / below |
| `Ctrl-a z` | Zoom (toggle fullscreen) for current pane |
| `Ctrl-a H/J/K/L` | Resize pane (repeatable) |
| `Ctrl-a [` | Copy mode (scroll; `v` select, `y` copy, `q` exit) |
| `Ctrl-a r` | Reload tmux config |
| `Ctrl-a S` | Toggle synchronize-panes |

### 0.2 Neovim modes

Neovim is modal — what a key does depends on the mode.

| Mode | Enter with | Purpose |
|---|---|---|
| **Normal** | `Esc` | Navigation and commands (default) |
| **Insert** | `i` `a` `o` `O` `s` `c…` | Type text |
| **Visual** | `v` (`V` line, `Ctrl-v` block) | Select |
| **Command** | `:` | Ex commands (`:w`, `:q`, …) |

```
Normal → Insert:  i (before)  a (after)  o (line below)  O (line above)
Normal → Visual:  v (char)    V (line)   Ctrl-v (block)
Insert → Normal:  Esc
```

### 0.3 Neovim movement (Normal mode)

Character/line: `h`/`l` left/right, `j`/`k` down/up, `0` line start, `^` first
non-blank, `$` line end, `gg`/`G` file start/end, `<n>G` go to line `n`.

Words: `w`/`b`/`e` next/prev/end of word; `W`/`B`/`E` WORD variants.

In-line jumps: `f{c}`/`F{c}` jump to char, `t{c}`/`T{c}` up-to char, `;`/`,`
repeat, `%` matching bracket.

Scrolling: `Ctrl-d`/`Ctrl-u` half page, `Ctrl-f`/`Ctrl-b` full page, `zz` center,
`zt`/`zb` top/bottom.

Search: `/pat` forward, `?pat` backward, `n`/`N` next/prev, `*`/`#` word under
cursor forward/backward.

### 0.4 Neovim editing — operators + motions

The power of Neovim is composing an **operator** with a **motion** or **text
object**:

```
d w   → delete to next word        c iw  → change inner word
y $   → yank to end of line        > }   → indent to next paragraph
d af  → delete around function     ci"   → change inside quotes
```

Operators: `d` delete, `c` change, `y` yank, `>`/`<` indent, `=` auto-indent,
`g~` toggle case.

Text objects: `iw`/`aw` word, `ip`/`ap` paragraph, `i"`/`a"` quotes, `i(`/`a(`
parens, `i{`/`a{` braces, `i[`/`a[` brackets, plus treesitter `if`/`af` function,
`ic`/`ac` class, `ib`/`ab` block, `ip`/`ap` parameter.

Common edits: `x` delete char, `r{c}` replace char, `dd` delete line, `D` to end
of line, `yy` yank line, `p`/`P` paste, `u` undo, `Ctrl-r` redo, `.` repeat, `J`
join lines, `>>`/`<<` indent line.

### 0.5 Splits and the command line

Splits: `<leader>-` below, `<leader>|` right, `<leader>=` equalize, `<C-h/j/k/l>`
move focus, `<leader>x` close buffer, `:q` close split.

Command line (`:`): `:w` save, `:q` quit, `:wq`/`:x` save+quit, `:q!` discard,
`:e <file>` open, `:bn`/`:bp` next/prev buffer, `:%s/old/new/g` replace all,
`:noh` clear highlight, `:help <topic>`.

### 0.6 The quick-start loop

Practice until automatic: open with `<leader>ff` → navigate with `/pat`, `n`,
`w`, `f{c}` → edit with `ciw`, `dd`, `o` → `u`/`Ctrl-r`/`.` → save `<leader>S` →
jump with `gd` → switch pane `Ctrl-a h/l`.

---

## Part 1 — Install karya and learn the IDE

### 1.1 Install karya

karya installs as a **single binary** and touches **none** of your existing
settings — no symlinks over `~/.zshrc`, `~/.tmux.conf`, or `~/.config/nvim`.

```bash
# From a release (recommended):
curl -fsSL https://github.com/drjzlyan/karya/releases/latest/download/install.sh | sh

# Or build from source:
git clone git@github.com:drjzlyan/karya.git && cd karya
make install          # -> ~/.local/bin/karya

karya install         # extract isolated configs, detect/fetch tools
```

Optional shell integration (adds `~/.local/bin` to PATH and `karya` niceties)
— **opt-in**, appended only if you choose:

```bash
eval "$(karya shellenv)"     # add to your shell rc yourself if you want it
```

Verify:

```bash
karya version
karya doctor          # checks tmux, nvim, git, ripgrep, agents, language tools…
```

> **Now try it — type:** `karya doctor`
>
> It reports the isolated paths karya uses and which tools are ready. This is the
> same check the command tutorial's first lesson runs.

### 1.2 Select programming languages and versions

```bash
karya lang            # interactive selector (languages + runtime versions)
karya lang list       # show current selection
karya lang all        # select all, latest stable
```

Common languages (JSON, YAML, Bash, Lua, TOML, Markdown) are always available.
Selectable: Python, Java, TypeScript, Go, C/C++, Rust. Versions are queried from
`mise` and installed into karya's **isolated** prefix — your global toolchains
are untouched. The selection is saved under karya's data dir.

### 1.3 Start the IDE session

```bash
cd ~/some/project
karya                 # or: karya dev myproj ~/some/project
```

You get a 3-pane layout plus a `git` window:

```
┌──────────────────────┬──────────────────┐
│                      │  agent           │
│      editor (nvim)   ├──────────────────┤
│                      │  build / test    │
└──────────────────────┴──────────────────┘
```

Move between panes with `Ctrl-a h/j/k/l`. The editor runs karya's Neovim
(`NVIM_APPNAME=karya/nvim`, fully isolated). Anything that opens `$EDITOR` (git,
lazygit, agents) routes back into the editor pane via `karya edit`.

> **Now try it — press:** `Ctrl-a l` to hop to the agent pane, then `Ctrl-a h` to
> come back to the editor. Then `Ctrl-a z` to zoom the editor pane full-screen,
> and `Ctrl-a z` again to restore the layout.

### 1.4 Core editor movements you'll use everywhere

Open a scratch file and practice the file/search/LSP keys:

| Key | Action |
|---|---|
| `<leader>E` / `<leader>O` | Explorer at file dir / working dir |
| `<leader>ff` / `<leader>fr` | Find files / recent files |
| `<leader><space>` | Switch buffers |
| `<leader>s/` / `<leader>s*` | Live grep / grep word under cursor |
| `<leader>st` | Search TODO/FIXME |
| `<leader>S` / `<leader>x` / `<leader>Z` | Save / close buffer / quit |
| `gcc` / `gc` | Toggle line / motion comment |
| `sa(` / `sd(` / `sr({` | Add / delete / replace surround |

Shared LSP keys (work once a server attaches): `gd` definition, `gD`
declaration, `gr` references, `gi` implementation, `gt` type, `K` hover,
`<C-k>` signature, `<leader>la` code action, `<leader>lr` rename, `<leader>lf`
format, `<leader>ls`/`<leader>ld` symbols.

**The `<leader>c` "Code" group is the same in every language.** Learn it once and
it works whether you're in Python, Go, Rust, Java, TypeScript, or C/C++ —
`<leader>ct` runs the nearest test, `<leader>cf` formats, `<leader>cc` builds,
`<leader>cp` runs the project, `<leader>cR` runs the file, `<leader>cr` refactors,
`<leader>ci` organizes imports, `<leader>cd` debugs the nearest test. karya
dispatches to whichever language owns the current buffer.

Press `<leader>` and pause — `which-key` shows every group.

> **Now try it — press:** `<leader>ff` to open the file finder (then `<Esc>` to
> dismiss it), `<leader>c` and pause to see the language-agnostic Code menu, and
> `K` with your cursor on a symbol to hover its docs.

### 1.5 Coding agents (the AI-first core)

karya detects agent CLIs on your `PATH` in preference order: `crush`, `claude`,
`codex`, `gemini`, `aider`, `copilot`.

```bash
karya                  # auto-detect; prompts if several are found
karya dev -a claude    # force a specific agent
karya dev -a none      # no agent, just a shell
```

The chosen agent is remembered **per project** and reused next time. Manage the
agent while you work, without leaving the editor:

| Key | Action |
|---|---|
| `Ctrl-a A` | Switch agent (interactive) → `karya agent switch` |
| `Ctrl-a N` | Cycle to the next agent → `karya agent next` |
| `Ctrl-a D` | Reset the layout (preserves nvim) → `karya agent reset` |
| `Ctrl-a Q` | Kill the session (confirm) |

From the shell (inside the session):

```bash
karya agent status     # current + available agents + saved preference
karya agent prefs      # all saved per-project preferences
karya agent clear      # forget this project's preference
```

`Ctrl-a D` is the escape hatch: if you break the layout, it rebuilds the default
and relaunches the current agent.

**Send context straight from the editor (the `<leader>a` group).** Instead of
copy-pasting into the agent, push what you're looking at into its pane — pasted
unsubmitted, so you review before hitting Enter:

| Key | Action |
|---|---|
| `<leader>aa` | Focus the agent pane |
| `<leader>ab` | Send the whole buffer |
| `<leader>as` | Send the visual selection (with `file:line`) |
| `<leader>ac` | Explain the code under cursor / selection |
| `<leader>ad` | Send the diagnostic on this line |
| `<leader>af` | Send a reference to the current file |

> **Now try it — press:** `<leader>ab` to send the whole buffer into the agent
> pane (it's pasted unsubmitted, so you review before hitting Enter), then
> `<leader>aa` to jump your cursor into the agent.

You're ready. Quit with `<leader>Z` and pick a language below. (Prefer to be
guided? `karya tutorial ide` walks every one of these keys for a language you
choose, detecting each keystroke as you go.)

### 1.6 Human-in-the-loop tasks

Beyond typing at one agent pane, karya runs agent work as **tasks** you direct
and review. A task is a **spec contract** — `.karya/tasks/<id>/SPEC.md` in your
repo: objective, machine-checkable acceptance criteria, and the verification
commands karya will run — executed in an isolated **git worktree** on a
`task/<id>` branch under karya's own directories. **Your real branch is never
touched until a human gate lets work merge.** This is task-level isolation,
layered on top of karya's zero-impact isolation.

```bash
karya init                              # one-time: scaffold .karya/ + AGENTS.md
karya task new add-health-endpoint      # scaffold the spec (opens in the editor)
karya task start 2026-08-05-add-health-endpoint   # create the isolated worktree
karya task list                         # the task board — or: karya tasks
karya task status                       # per-state counts + the gate inbox
karya task show <id>                    # state, spec summary, gate history
karya task abandon <id>                 # teardown: worktree + branch + artifacts
```

Only `SPEC.md` is meant to be committed from `.karya/` — the rest (state,
plans, evidence) is local runtime state. Tasks advance through mandatory
**human gates** (plan, diff, verification), each recorded in the task's
`STATE.json` so you can always answer "who approved this, when, with what
feedback". The agent-driven steps (`karya plan`, `implement`, `verify`,
`merge`) arrive with the upcoming adapter layer.

**From the editor and tmux:**

| Key | Action |
|---|---|
| `<leader>kn` | New task (prompts for a slug; scaffolds the spec) |
| `<leader>kl` | List tasks (the task board) |
| `<leader>ks` | Show a task's state, spec summary, and gate history |
| `<leader>kt` | Start a task (create its isolated worktree) |
| `<leader>ka` | Abandon a task |
| `Ctrl-a T` | Task board popup |

Because each task is its own worktree, you can run **several at once** — each
reviewed on its own terms.

**The built-in agent (optional).** With `ANTHROPIC_API_KEY` set, `karya agent
native` runs karya's own Claude-API agent, which **pauses for your approval on
every file write and shell command** — a per-action permission prompt that
external agent CLIs can't offer. BYO-CLI agents stay the default.

> **Want to see the isolation for real?** `karya tutorial` includes a
> self-checking lesson that creates and tears down a real task worktree so you can
> watch the containment work on your own machine.

---

## Part 2 — Python

Goal: scaffold a project, write a function + test, get LSP + format-on-save, run
tests, and debug with DAP.

```bash
karya new python calc
cd calc
uv venv                       # .venv (auto-detected)
uv add --dev debugpy          # for debugging (2.4)
karya                         # open the IDE here
```

> **Now try it — type:** `karya new python calc`
>
> Every language below follows the same shape — `karya new <lang> <name>`, then
> `karya` to open the IDE. The `<leader>c` Code group (`<leader>cc` build,
> `<leader>cT` test, `<leader>ct` nearest test) is *identical* in all of them, so
> the IDE tutorial only walks the one language you pick.

Write `src/calc/main.py`:

```python
def add(a: int, b: int) -> int:
    return a + b

def divide(a: int, b: int) -> float:
    if b == 0:
        raise ValueError("b must not be zero")
    return a / b
```

Add `tests/test_calc.py`:

```python
import pytest
from calc.main import add, divide

def test_add():
    assert add(2, 3) == 5

def test_divide_by_zero():
    with pytest.raises(ValueError):
        divide(1, 0)
```

**Save** with `<leader>S`. In Python buffers, save organizes imports and formats
with Ruff automatically. `basedpyright` provides diagnostics/navigation.

**Run & test** — the shared `<leader>c` group, active in Python buffers:

| Key | Action |
|---|---|
| `<leader>cR` | Run current file |
| `<leader>cT` | pytest current file |
| `<leader>ct` | pytest function/method under cursor |
| `<leader>ci` / `<leader>cf` | Organize imports / format (Ruff) |
| `<leader>cm` / `<leader>cs` | Run module / run selection (visual) |
| `<leader>cv` | Show the detected interpreter |

Whole-project tests: `:PythonTestProject`.

Test output appears in the tmux build/test pane.

**Debug** (`debugpy`, keys under `<leader>d`): cursor on `return a + b` →
`<leader>db` (breakpoint) → `<leader>dc` (start). Step with `<leader>do`/`di`/`dO`,
toggle UI `<leader>du`, REPL `<leader>dr`, stop `<leader>dt`, conditional
breakpoint `<leader>dB`.

---

## Part 3 — Java

Goal: open a Maven project, let jdtls index it, navigate/refactor, run and debug
JUnit tests.

```bash
karya new java com.example.calc
cd calc
karya
```

Open any `*.java` file — `jdtls` starts lazily with a per-project workspace under
karya's cache and reuses it across sessions (first open indexes, later opens are
fast). CodeLens shows reference/implementation counts.

Write `src/main/java/com/example/App.java`:

```java
package com.example;

public class App {
    public static int add(int a, int b) { return a + b; }
    public static double divide(int a, int b) {
        if (b == 0) throw new IllegalArgumentException("b must not be zero");
        return (double) a / b;
    }
}
```

Add `src/test/java/com/example/AppTest.java` (JUnit 5):

```java
package com.example;

import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

class AppTest {
    @Test void adds() { assertEquals(5, App.add(2, 3)); }
    @Test void dividesByZero() {
        assertThrows(IllegalArgumentException.class, () -> App.divide(1, 0));
    }
}
```

Format on save runs `google-java-format`. The shared `<leader>c` group in Java:

| Key | Action |
|---|---|
| `<leader>ci` / `<leader>cf` | Organize imports / format |
| `<leader>cc` / `<leader>cp` | Build / package project |
| `<leader>ct` / `<leader>cT` | Run nearest test / test class |
| `<leader>cd` / `<leader>cD` | Debug nearest test / test class |
| `<leader>cr` / `<leader>ch` | Refactor / incoming calls |
| `<leader>cw*` | Workspace: `cwb` build, `cwr` reload, `cww` restart jdtls, `cwc` clear cache, `cwl` logs, `cwh` type hierarchy, `cwv` verify |

Refactor commands: `:JavaExtractMethod`, `:JavaExtractVariable`,
`:JavaInlineVariable`, `:JavaMoveType`, and more. Navigate with the shared LSP
keys (`gd`, `gr`, `<leader>lr` to rename across the project).

Workspace management under `<leader>cw`: `<leader>cww` restart jdtls, `<leader>cwc`
clear cache, `<leader>cwl` logs — use these if imports/rename look stale.

---

## Part 4 — TypeScript / JavaScript

```bash
karya new typescript calc-ts
cd calc-ts
karya
```

Scaffold: `package.json` (ESM), `tsconfig.json` (strict), `src/index.ts`,
`test/index.test.ts` (Node's built-in test runner); npm deps installed.

`src/index.ts`:

```typescript
export function add(a: number, b: number): number { return a + b; }
export function divide(a: number, b: number): number {
  if (b === 0) throw new Error("b must not be zero");
  return a / b;
}
```

`ts_ls` attaches automatically; format on save runs through it. The shared
`<leader>c` group applies: `<leader>ct` nearest `node:test`, `<leader>ci`
organize imports, `<leader>cc` build, `<leader>cT` test file, `<leader>cp` start,
`<leader>cf` format. Shared LSP keys work identically (`gd`, `gr`, `<leader>la`,
`<leader>lr`).

---

## Part 5 — Go

```bash
karya new go example.com/calc
cd calc
karya
```

`gopls` attaches on Go files; save organizes imports (`goimports`) then formats
(`gofmt`). Write a package + `_test.go`, then:

| Action | Key |
|---|---|
| Build | `<leader>cc` → `go build ./...` |
| Test | `<leader>cT` → `go test ./...` |
| Run | `<leader>cp` → `go run .` |
| Nearest test | `<leader>ct` |
| Organize imports | `<leader>ci` (or `<leader>lI`) |

**Debug with Delve** (`<leader>d*`): breakpoint `<leader>db`, start `<leader>dc`
(pick Debug file / package / test / attach), step `<leader>do`/`di`/`dO`. Same
DAP keys as Python and Java.

---

## Part 6 — C / C++

```bash
karya new cpp calc-cpp
cd calc-cpp && mkdir build
karya
```

Scaffold: `CMakeLists.txt` (C++20 + test target), `src/main.cpp`,
`tests/test_main.cpp`. Generate the compile database so `clangd` is accurate:

```bash
cd build && cmake -DCMAKE_EXPORT_COMPILE_COMMANDS=ON ..
```

Reopen the file; `clangd` provides diagnostics, clang-tidy, and completion.
Format on save runs `clang-format`. Build/test via tasks:

| Action | Key |
|---|---|
| Build | `<leader>cc` → `cmake --build build` |
| Test | `<leader>ct` → `ctest --output-on-failure` |
| Clean | `:CppBuild` / task layer |

Shared LSP keys apply. C/C++ uses the same `<leader>c` group as every language.

---

## Part 7 — Rust

```bash
karya new rust calc-rs
cd calc-rs
karya
```

`rust-analyzer` attaches (clippy on save, inlay hints, proc-macro support).
Write code plus a `#[cfg(test)]` module, then:

| Action | Key |
|---|---|
| Build | `<leader>cc` → `cargo build` |
| Test | `<leader>cT` → `cargo test` |
| Run | `<leader>cp` → `cargo run` |
| Nearest test | `<leader>ct` |

Format on save runs `rustfmt`. Shared LSP keys apply; Rust uses the same
`<leader>c` group as every language.

---

## Part 8 — Git and lazygit

Git tooling loads only inside a repo. Initialize one if needed
(`git init && git add -A && git commit -m "init"`).

**In-editor hunk work** (gitsigns): `<leader>gn`/`<leader>gp` next/prev hunk,
`<leader>gh` preview, `<leader>gb` blame, `<leader>gs`/`<leader>gr` stage/reset,
`<leader>gu` undo stage, `<leader>gD` diff vs index.

**Review changes** (diffview): `<leader>gd` opens all current changes;
`:DiffviewOpen branch...HEAD`, `:DiffviewFileHistory %`, `:DiffviewClose`.

**Ship with the agent** (`<leader>gc` / `Ctrl-a G` / `karya ship`): stages the work
tree, has the active coding agent write a Conventional-Commit message from the
staged diff, shows it for confirmation, then commits — add `--push` to push and
`--pr` to open a pull request. `Ctrl-a G` runs it in a popup and pushes. This is
the scaffold → implement → ship loop finished from inside the IDE.

**Manual commit/push/branch — lazygit** (`Ctrl-a g`): opens the dedicated `git`
window (reuses it if open). Panels `1`–`5` (Status/Files/Branches/Commits/Stash).
Keys: `<space>` stage/unstage, `a` stage all, `c` commit, `P` push, `p` pull, `e`
edit in the editor pane, `?` all keys, `q` quit back to the dev window.

A full round-trip: edit in nvim → `<leader>gd` review → `<leader>gc` agent-ship
(or `Ctrl-a g` → stage → `c` commit → `P` push → `q` back).

> **Now try it — press:** `<leader>gd` to review your changes in diffview, then
> `Ctrl-a g` to open lazygit and `Ctrl-a p` to return. When you're ready to
> finish the loop, `<leader>gc` (or `Ctrl-a G`) lets the agent write the commit
> message and ships it.

---

## Quick reference cheat sheet

| Want to… | Do this |
|---|---|
| Learn the CLI hands-on | `karya tutorial` (you type it; a sandbox verifies) |
| Learn the IDE hands-on | `karya tutorial ide [lang]` (or `:KaryaTutorial` / `<leader>?`) |
| Start / attach the IDE | `karya` (or `karya dev -a claude`) |
| See all key groups | press `<leader>` and pause |
| Find a file / grep project | `<leader>ff` / `<leader>s/` |
| Definition / references / rename | `gd` / `gr` / `<leader>lr` |
| Format the buffer | `<leader>lf` |
| New project | `karya new <lang> <name>` or `Ctrl-a P` |
| Switch coding agent | `Ctrl-a A` (pick) or `Ctrl-a N` (cycle) |
| Send buffer / selection to the agent | `<leader>ab` / `<leader>as` |
| Run current file / test under cursor (any language) | `<leader>cR` / `<leader>ct` |
| Debug (Python/Java/Go) | `<leader>db` then `<leader>dc` |
| Build / test any project (any language) | `<leader>cc` / `<leader>cT` |
| Ship: agent commit (& push) | `<leader>gc` / `Ctrl-a G` / `karya ship` |
| Open lazygit / review changes | `Ctrl-a g` / `<leader>gd` |
| Select or change languages | `karya lang` |
| Check the environment | `karya doctor` |
| Update everything | `karya update` |

See also [keymaps.md](keymaps.md) for the full key reference.
