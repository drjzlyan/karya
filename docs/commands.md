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

## Tasks (human-in-the-loop, agents-first)

Each task runs in an isolated git worktree on a `karya/<id>` branch; your real
branch is untouched until you merge. Inside a task's session the `<id>` is
optional — it defaults to the current task.

| Command | What it does |
|---|---|
| `karya task new "<prompt>" [--agent <name>] [--plan]` | Start a task (isolated worktree + branch + agent). `--plan` drafts a plan and holds for approval |
| `karya task list` (or `karya tasks`) | List the project's tasks and their status |
| `karya task dashboard` | Fleet view — pick a task to switch to (also `Ctrl-a T`) |
| `karya task switch <id>` | Attach to a task's session (rooted at its worktree) |
| `karya task plan <id>` | Show the drafted plan |
| `karya task approve-plan <id>` | Approve the plan → the task starts work |
| `karya task review [<id>]` | Show the task's diff vs. its base — the pre-apply review |
| `karya task merge [<id>] [--push]` | Commit + merge the task branch into your branch (permission-gated) |
| `karya task reject [<id>]` | Mark the task rejected (worktree kept for inspection) |
| `karya task checkpoint [<id>] [label]` | Record a restorable snapshot of the worktree |
| `karya task rewind [<id>] [index\|sha]` | Reset the worktree to a checkpoint |
| `karya task allow <merge\|push\|rewind>` | Pre-authorize a karya action so it stops prompting |
| `karya task rm <id> [-y]` | Remove the task: its worktree, branch, and record |

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
