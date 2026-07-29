# assets — embedded single-binary payload

These files are embedded into the karya binary via `go:embed` and extracted to
karya-owned directories at install/update time. This is what makes karya a true
single binary while still shipping the full Neovim + tmux experience.

Planned contents:

- `nvim/` — a vendored copy of the Neovim config (from `../../nvim-config`),
  synced by a build step. Git-ignored here; populated during the build.
  Extracted to `~/.config/karya/nvim` and loaded via `NVIM_APPNAME=karya`.
- `tmux.conf` — karya's tmux configuration, used with a dedicated socket
  (`tmux -L karya -f …`) so the user's `~/.tmux.conf` is never sourced.
- `starship.toml` — optional prompt sample (never overwrites the user's).
- `manifest.json` — asset version; drives re-extraction on `karya update`.

Nothing here is wired up yet — see ROADMAP Phase 1 (tmux.conf) and Phase 3
(nvim vendoring + embed).
