# Command reference

Every `karya` command, grouped by what it does. Read this offline any time with
`karya docs commands`, or get focused help with `karya help <command>`.

> Flags come **before** positionals: `karya dev -a claude myproj`, not
> `karya dev myproj -a claude`.

---

## Session

| Command | What it does |
|---|---|
| `karya` | Launch or attach the IDE session for the current directory |
| `karya dev [name] [path]` | Explicit session launch |
| `karya dev -a, --agent <name\|none>` | Choose the coding agent (`none` = plain shell) |
| `karya dev -k, --kill` | Kill an existing session and recreate it |
| `karya dev -q, --quit` | Quit (kill) the session cleanly |
| `karya edit <file> [line]` | Open a file in the editor pane (also karya's `$EDITOR`) |
| `karya run [-d dir] <cmd…>` | Run a command in the build/test pane |
| `karya run --focus` | Focus the build/test pane (no command) |

## Coding agents

| Command | What it does |
|---|---|
| `karya agent status` | Current + available agents and the saved preference |
| `karya agent switch` / `next` / `prev` | Change or cycle the session's agent |
| `karya agent reset` | Rebuild the pane layout (preserves the editor) |
| `karya agent focus` | Jump to the agent pane |
| `karya agent send [--file f --line n --label t]` | Paste stdin into the agent pane (the editor↔agent bridge) |
| `karya agent native [prompt]` | Run karya's **built-in** Claude-API agent — approves each file write / command. Needs `ANTHROPIC_API_KEY` (model via `KARYA_AGENT_MODEL`) |
| `karya agent prefs` / `clear` | Show / forget the per-project agent preference |

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

Tasks advance through mandatory human gates — plan, diff, verification —
recorded in `STATE.json`. The agent-driven steps (`karya plan`, `implement`,
`verify`, `merge`) arrive with the upcoming adapter layer.

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
| `karya tutorial [n \| ide [lang]]` | The self-working tutorial (verified against a throwaway sandbox); `ide` runs the in-editor walkthrough |
| `karya docs [topic]` | Read the embedded docs offline (no topic lists them) |
| `karya keys` | The full CLI / tmux / Neovim key reference |
| `karya help [command]` | This help, or detailed help for one command |
