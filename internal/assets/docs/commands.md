# Command reference

Every `karya` command, grouped by what it does. Read this offline any time with
`karya docs commands`, or get focused help with `karya help <command>`.

> karya is a **single-process TUI IDE**: running `karya` opens its own screen —
> window/pane manager, editor (embedded Neovim), git panel, and task/review views
> — all under one keymap ([keymaps.md](keymaps.md)). There is no external
> multiplexer to attach to.

> Flags come **before** positionals: `karya task new -a claude my-slug`, not
> `karya task new my-slug -a claude`.

---

## The IDE

| Command | What it does |
|---|---|
| `karya` | Launch the single-process TUI IDE for the current directory |
| `karya edit <file> [line]` | Open a file in the embedded editor pane (also karya's `$EDITOR`) |

Inside the IDE, everything is keyboard-driven under the `Ctrl-Space` leader —
panes, tabs, git, tasks, gates, and the embedded editor. See
[keymaps.md](keymaps.md).

## Coding agents

Agents are configured per task (the spec's `agent:` field, or per-step pins) and
run either headlessly (`karya plan`/`implement`) or interactively in a PTY pane
bound to the task worktree.

| Command | What it does |
|---|---|
| `karya doctor` | Shows detected agents (`crush`/`claude`/`codex`/`gemini`/`aider`/`copilot`) |
| `karya plan <id>` | Run the plan step with the task's agent (headless) |
| `karya implement <id>` | Run/continue the implement step with the task's agent |
| `karya gate delegate <id> --to <agent>` | Delegate a gate crossing to an agent (recorded) |

## Tasks (human-in-the-loop)

A task is a **spec contract** (`.karya/tasks/<id>/SPEC.md` in your repo — the
one karya artifact meant to be committed) executed in an isolated git worktree
on a `task/<id>` branch; your real branch is untouched until a human gate lets
work merge. State and gate history live in `STATE.json` next to the spec.

| Command | What it does |
|---|---|
| `karya init [--force]` | Scaffold `.karya/` + a repo `AGENTS.md` (build/test/lint contract) from detected toolchains |
| `karya task new <slug> [--agent <name>]` | Scaffold a task spec at `.karya/tasks/<date>-<slug>/SPEC.md` (opens in the editor pane inside a session) |
| `karya task list` (or `karya tasks`) | The task board: every task with state, agent, and title (also `Ctrl-a T`) |
| `karya task status` | Per-state counts plus the gate inbox (tasks waiting on a human) |
| `karya task show <id>` | State, workspace, spec summary, and the full gate history |
| `karya task start <id> [--base <ref>]` | Create the task's worktree on branch `task/<id>` forked from the base ref (default `HEAD`) |
| `karya task abandon <id> [-y]` | Teardown: remove the worktree, the branch, and the task's artifacts |
| `karya task audit <id>` | Gate history: who/what approved what, and when |

Tasks advance through mandatory human gates — plan, diff, verification —
recorded in `STATE.json`.

## Gates, review & merge

| Command | What it does |
|---|---|
| `karya plan <id>` | (Re)run the plan step; produces `PLAN.md` for review |
| `karya implement <id>` | Run/continue the implement step in the task worktree |
| `karya review <id>` | Open the review layout for the pending gate (spec · artifact · feedback) |
| `karya gate list` | The gate inbox: tasks waiting on a human |
| `karya gate approve\|reject <id>` | Cross a gate (reject requires feedback; loops back to the agent) |
| `karya gate delegate <id> --to <agent>` | Let an agent cross the gate (recorded in `STATE.json`) |
| `karya verify <id>` | Run the spec's `Verification` block; record evidence |
| `karya merge <id>` | Merge the task branch (post-verify gate) or open a PR |

## Projects & languages

| Command | What it does |
|---|---|
| `karya new <lang> <name> [dir]` | Scaffold a project (python \| java \| typescript \| go \| cpp \| rust) |
| `karya ship [--push] [--pr] [--no-verify] [-y]` | Stage, let the agent write the commit message, then commit (& push / open a PR) |
| `karya lang` | Interactive language + runtime-version selector |
| `karya lang list` / `add <lang> [versions]` / `remove <lang>` / `all` | Manage the selected languages and their tooling |
| `karya profile list` / `install <id>` | Managed tool profiles (core \| docs \| per-language) |
| `karya tool list` / `update <id>\|all` | Managed-tool health and updates |

## Lifecycle

| Command | What it does |
|---|---|
| `karya install` | Set up karya (isolated, non-destructive): extract configs, fetch tools |
| `karya update [--check]` | Self-update the binary, configs, tools, and editor plugins |
| `karya uninstall` | Remove karya entirely — nothing else is touched |
| `karya doctor` | Health check: tools, versions, isolation, per-language tooling |
| `karya shellenv` | Print opt-in shell integration (`eval "$(karya shellenv)"`) |
| `karya completion <bash\|zsh\|fish>` | Print a shell completion script to source |
| `karya version` | Version / build info |

## Learn & help

| Command | What it does |
|---|---|
| `karya tutorial [n]` | The self-working tutorial (verified against a throwaway sandbox) |
| `karya docs [topic]` | Read the embedded docs offline (no topic lists them) |
| `karya keys` | The full unified keymap reference (single `Ctrl-Space` leader) |
| `karya help [command]` | This help, or detailed help for one command |
