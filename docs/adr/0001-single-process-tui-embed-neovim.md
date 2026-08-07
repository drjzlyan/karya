# ADR 0001 — Single-process TUI; embed Neovim as the editing engine

- Status: Accepted
- Date: 2026-08-06
- Deciders: project owner + Claude Code
- Supersedes: the orchestrator architecture (tmux + standalone Neovim UI + lazygit)

## Context

karya v0 and the first v1.0 design were **orchestrators**: karya shelled out to
tmux as the window manager (prefix `Ctrl-a`), ran Neovim as a standalone editor
(leader `Space`), and wrapped lazygit as the git UI. Each tool shipped its own
independent keymap layer.

This produced three problems the owner wanted fixed:

1. **Fragmented keymaps.** Pane navigation, resizing, tab switching, git, and
   editor actions each lived in a different tool with a different prefix/leader.
   Muscle memory did not transfer; there was no single grammar.
2. **Three UIs pretending to be one IDE.** Composition happened at the tmux pane
   boundary, so karya could not present consistent chrome, discovery, or review
   surfaces across the whole IDE.
3. **Hard to test as one product.** Behavior spanned tmux config, Lua config, and
   lazygit — none of it exercised by karya's own Go test suite.

The owner asked to "build the complete IDE from scratch" as a single tool, with
one leader and consistent keymaps everywhere, and to keep a single highly
customizable tool only where it earns its place.

## Decision

**karya becomes a single process that owns the terminal**, and **Neovim is
embedded as the text-editing engine** over msgpack-RPC (`nvim --embed`).

- karya draws its own screen: window/pane/tab manager (replaces tmux), git panel
  (replaces lazygit), task/gate/review views, and PTY-hosted shells and agent
  CLIs. It renders to its own cell buffer and diffs frames to the terminal.
- Neovim runs headless as an engine; karya consumes its `redraw` grid and renders
  it inside an editor pane, forwarding input via `nvim_input`. Neovim keeps
  LSP/treesitter/completion/text-editing; it presents **no UI or keymap surface**
  of its own.
- **One keymap engine, one leader (`Ctrl-Space`).** karya intercepts every key,
  dispatches IDE actions, and forwards only unclaimed keys to the focused pane.
- **Stdlib only.** The cell buffer + diff renderer, ANSI/terminfo terminal I/O,
  PTY host, and msgpack-RPC client are all built from scratch in Go stdlib. No
  bubbletea, no pty library, no msgpack library.

Two alternatives were explicitly considered and rejected:

- **Build the text editor from scratch too** (no Neovim). Rejected: re-implementing
  modal editing, LSP integration, treesitter, and per-language tooling is months
  of work with no differentiation. Neovim as an embedded engine is the "single
  highly customizable tool" that earns its keep.
- **Adopt a TUI/PTY/msgpack library** to reduce the from-scratch surface.
  Rejected: violates the locked zero-external-dependency rule and the single
  static binary promise. Revisit only via a future ADR if the stdlib approach
  proves untenable.

## Consequences

Positive:

- One consistent, discoverable keymap across the entire IDE; muscle memory
  transfers everywhere.
- The whole UI is karya's own code, testable at four levels (pure model tests,
  cell-buffer golden snapshots, PTY integration, e2e) with fakes for nvim/pty/
  agents/terminal (DESIGN.md §8).
- Consistent chrome, which-key discovery, and review surfaces; no pane-boundary
  seams.
- Single static binary, no external multiplexer or git TUI to install.

Negative / costs:

- Significant new from-scratch surface: terminal I/O, cell buffer, PTY host, VT
  parser, msgpack-RPC client, and a window/pane manager — all to build and test.
- Grid-rendering fidelity for the embedded editor (wide chars, highlights,
  cmdline/messages) must be handled carefully; mitigated by `ext_linegrid`,
  recorded-redraw snapshot tests, and a real-Neovim integration test.
- Portability work for termios and PTYs across darwin/linux (build-tagged files).

Reused unchanged: the headless workflow engine — `task`, `spec`, `worktree`,
`agentrun`, `prompts`, `config` (isolation), `tools`. Removed: `internal/tmuxx`,
`internal/session`, `internal/editor`, `internal/assets/tmux.conf`, and the
UI/keymap subtree of `internal/assets/nvim` (config slims to an engine config).

Rollout is phased (ROADMAP.md): docs first, then a walking-skeleton TUI, then the
editor embed, then panes/git/views — the binary stays launchable at every phase
and the headless engine never regresses.
