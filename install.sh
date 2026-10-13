#!/bin/sh
# Install docket from GitHub releases:
#   curl -fsSL https://raw.githubusercontent.com/jamesngdev/docket/master/install.sh | sh
# Env overrides: DOCKET_VERSION=v0.1.0 (default: latest), DOCKET_INSTALL_DIR=~/.local/bin
set -eu

REPO="jamesngdev/docket"
INSTALL_DIR="${DOCKET_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${DOCKET_VERSION:-latest}"

fail() { echo "docket install: $*" >&2; exit 1; }

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) fail "unsupported OS: $(uname -s) (macOS and Linux only)" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) fail "unsupported architecture: $(uname -m)" ;;
esac

command -v curl >/dev/null || fail "curl is required"
command -v tar >/dev/null || fail "tar is required"

asset="docket_${os}_${arch}.tar.gz"
if [ "$VERSION" = latest ]; then
  base="https://github.com/$REPO/releases/latest/download"
else
  base="https://github.com/$REPO/releases/download/$VERSION"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "Downloading $asset ($VERSION)..."
curl -fsSL "$base/$asset" -o "$tmp/$asset" || fail "download failed: $base/$asset"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt" || fail "checksum download failed"

expected="$(grep " $asset\$" "$tmp/checksums.txt" | awk '{print $1}')"
[ -n "$expected" ] || fail "no checksum for $asset"
if command -v sha256sum >/dev/null; then
  actual="$(sha256sum "$tmp/$asset" | awk '{print $1}')"
else
  actual="$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')"
fi
[ "$expected" = "$actual" ] || fail "checksum mismatch for $asset"

tar -xzf "$tmp/$asset" -C "$tmp" docket
mkdir -p "$INSTALL_DIR"
# Replace via rename so a running daemon keeps its old inode and restarts cleanly.
mv -f "$tmp/docket" "$INSTALL_DIR/docket.new"
chmod +x "$INSTALL_DIR/docket.new"
mv -f "$INSTALL_DIR/docket.new" "$INSTALL_DIR/docket"

echo "Installed $("$INSTALL_DIR/docket" version) to $INSTALL_DIR/docket"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "Note: $INSTALL_DIR is not on your PATH. Add this to your shell profile:"
     echo "  export PATH=\"$INSTALL_DIR:\$PATH\"" ;;
esac
echo "Runtime deps: git, gh, claude, tmux (docker, codegraph optional). Run 'docket' to start."
