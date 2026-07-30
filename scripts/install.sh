#!/bin/sh
# karya installer — downloads the right prebuilt binary from GitHub Releases,
# verifies its checksum, installs it to ~/.local/bin/karya, and runs the isolated
# `karya install` setup. It never touches your shell rc, Homebrew, or global mise.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/drjzlyan/karya/main/scripts/install.sh | sh
#
# Environment overrides:
#   KARYA_VERSION   install a specific tag (e.g. v0.2.0); default: latest release
#   KARYA_BIN_DIR   install location (default: ~/.local/bin)
#   KARYA_NO_SETUP  set to 1 to skip running `karya install` after download

set -eu

REPO="drjzlyan/karya"
BIN_DIR="${KARYA_BIN_DIR:-$HOME/.local/bin}"

log() { printf 'karya: %s\n' "$*"; }
die() { printf 'karya: error: %s\n' "$*" >&2; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"; }

need curl
need tar

# Detect platform in goreleaser's naming (GOOS/GOARCH).
os=$(uname -s)
case "$os" in
  Darwin) os=darwin ;;
  Linux)  os=linux ;;
  *) die "unsupported OS: $os (karya ships macOS and Linux builds)" ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) die "unsupported architecture: $arch" ;;
esac

# Resolve the version to install.
tag="${KARYA_VERSION:-}"
if [ -z "$tag" ]; then
  log "Resolving latest release..."
  tag=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep '"tag_name"' | head -n1 | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
  [ -n "$tag" ] || die "could not determine the latest release tag"
fi
version=${tag#v}

asset="karya_${version}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/$tag"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

log "Downloading $asset ($tag)..."
curl -fsSL "$base/$asset" -o "$tmp/$asset" || die "download failed: $base/$asset"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt" || die "checksums download failed"

# Verify the SHA-256 against the published checksums.
log "Verifying checksum..."
want=$(grep " $asset\$" "$tmp/checksums.txt" | awk '{print $1}')
[ -n "$want" ] || die "no checksum found for $asset"
if command -v sha256sum >/dev/null 2>&1; then
  got=$(sha256sum "$tmp/$asset" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  got=$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')
else
  die "no sha256 tool found (need sha256sum or shasum)"
fi
[ "$got" = "$want" ] || die "checksum mismatch (got $got, want $want)"

log "Installing to $BIN_DIR/karya..."
mkdir -p "$BIN_DIR"
tar -xzf "$tmp/$asset" -C "$tmp"
# The archive may nest the binary under a directory; find it.
binpath=$(find "$tmp" -type f -name karya | head -n1)
[ -n "$binpath" ] || die "archive did not contain a karya binary"
install -m 0755 "$binpath" "$BIN_DIR/karya"

if [ "${KARYA_NO_SETUP:-0}" != "1" ]; then
  log "Running isolated setup (karya install)..."
  "$BIN_DIR/karya" install || log "setup reported an issue; you can re-run 'karya install' later"
fi

log "Installed $tag to $BIN_DIR/karya"
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) log "Add $BIN_DIR to your PATH, or run: eval \"\$($BIN_DIR/karya shellenv)\"" ;;
esac
log "Launch the IDE with: karya"
