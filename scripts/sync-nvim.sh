#!/usr/bin/env bash
# sync-nvim.sh — vendor the Neovim config into karya for embedding.
#
# karya ships the full Neovim IDE inside its single binary via go:embed. The
# authoritative source of that config is the sibling `nvim-config` repo; this
# script copies the *whitelisted* subset (the runtime Lua config and its plugin
# lockfile) into internal/assets/nvim/, which internal/assets embeds. Run it
# whenever the upstream config changes, then commit the refreshed tree.
#
# Usage: scripts/sync-nvim.sh [source-dir]
#   source-dir defaults to ../nvim-config relative to the repo root.
#
# Only files needed at runtime are vendored — docs, tests, CI, VCS metadata and
# the tool-install scripts are intentionally excluded so the embedded payload
# stays minimal and reproducible.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
src="${1:-$repo_root/../nvim-config}"
dest="$repo_root/internal/assets/nvim"

if [[ ! -f "$src/init.lua" ]]; then
	echo "sync-nvim: no nvim-config at '$src' (expected init.lua)" >&2
	exit 1
fi

# Whitelist of top-level entries to vendor. Everything else is excluded.
entries=(init.lua lazy-lock.json lua)

echo "sync-nvim: $src -> $dest"
rm -rf "$dest"
mkdir -p "$dest"

for entry in "${entries[@]}"; do
	if [[ ! -e "$src/$entry" ]]; then
		echo "sync-nvim: missing '$entry' in source" >&2
		exit 1
	fi
	cp -R "$src/$entry" "$dest/$entry"
done

# `after/` is an (empty) runtime override dir in the source; recreate it with a
# keep-file so the layout matches and go:embed has something to include.
mkdir -p "$dest/after"
: >"$dest/after/.gitkeep"

# Strip anything that slipped in (e.g. macOS metadata) so the tree is clean.
find "$dest" -name '.DS_Store' -delete

count=$(find "$dest" -type f | wc -l | tr -d ' ')
echo "sync-nvim: vendored $count files"
