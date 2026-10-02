#!/usr/bin/env bash
# Installs the latest mcpscope binary for your OS/arch from GitHub Releases.
set -euo pipefail

REPO="wangzi5151/mcpscope"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  aarch64|arm64) arch="arm64" ;;
  *) echo "unsupported arch: $arch" >&2; exit 1 ;;
esac
case "$os" in
  linux|darwin) ext="" ;;
  msys*|mingw*|cygwin*|windows*) os="windows"; ext=".exe" ;;
  *) echo "unsupported os: $os" >&2; exit 1 ;;
esac

BIN="mcpscope-${os}-${arch}${ext}"
URL="https://github.com/${REPO}/releases/latest/download/${BIN}"
DEST="${DEST:-/usr/local/bin}"

echo "downloading ${URL} ..."
tmp=$(mktemp)
curl -fsSL "$URL" -o "$tmp"
chmod +x "$tmp"

if [ -w "$DEST" ]; then
  mv "$tmp" "${DEST}/mcpscope${ext}"
else
  echo "need sudo to install into ${DEST}"
  sudo mv "$tmp" "${DEST}/mcpscope${ext}"
fi
echo "installed: $(command -v mcpscope || echo "${DEST}/mcpscope${ext}")"
"${DEST}/mcpscope${ext}" --version
