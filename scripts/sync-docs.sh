#!/usr/bin/env bash
# sync-docs.sh — vendor the user docs into karya for embedding.
#
# karya ships its user documentation inside its single binary via go:embed so
# `karya help`, `karya docs` and `karya tutorial` work fully offline with only
# the binary on PATH. The authoritative source is docs/*.md at the repo root;
# this script copies them into internal/assets/docs/, which internal/assets
# embeds. Run it whenever docs/*.md change, then commit the refreshed tree.
# A drift test (internal/assets) fails CI if the vendored copy falls out of sync.
#
# Usage: scripts/sync-docs.sh
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
src="$repo_root/docs"
dest="$repo_root/internal/assets/docs"

shopt -s nullglob
docs=("$src"/*.md)
if [[ ${#docs[@]} -eq 0 ]]; then
	echo "sync-docs: no *.md found in '$src'" >&2
	exit 1
fi

echo "sync-docs: $src -> $dest"
rm -rf "$dest"
mkdir -p "$dest"

for doc in "${docs[@]}"; do
	cp "$doc" "$dest/$(basename "$doc")"
done

# Strip anything that slipped in (e.g. macOS metadata) so the tree is clean.
find "$dest" -name '.DS_Store' -delete

count=$(find "$dest" -type f | wc -l | tr -d ' ')
echo "sync-docs: vendored $count files"
