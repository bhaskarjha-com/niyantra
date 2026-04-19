#!/bin/sh
# Niyantra installer — downloads the latest release binary for your platform.
# Usage: curl -fsSL https://raw.githubusercontent.com/bhaskarjha-com/niyantra/main/install.sh | sh
set -e

REPO="bhaskarjha-com/niyantra"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY="niyantra"

# Detect OS and architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Error: Unsupported architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
  linux) OS="linux" ;;
  darwin) OS="darwin" ;;
  *) echo "Error: Unsupported OS: $OS. For Windows, download from GitHub Releases."; exit 1 ;;
esac

# Get latest release tag
echo "Fetching latest release..."
TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')

if [ -z "$TAG" ]; then
  echo "Error: Could not determine latest release. Check https://github.com/${REPO}/releases"
  exit 1
fi

VERSION="${TAG#v}"
ARCHIVE="${BINARY}_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ARCHIVE}"

echo "Downloading ${BINARY} ${TAG} for ${OS}/${ARCH}..."
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

curl -fsSL "$URL" -o "${TMPDIR}/${ARCHIVE}"
tar -xzf "${TMPDIR}/${ARCHIVE}" -C "$TMPDIR"

# Install
if [ -w "$INSTALL_DIR" ]; then
  mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
else
  echo "Installing to ${INSTALL_DIR} (requires sudo)..."
  sudo mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
fi

chmod +x "${INSTALL_DIR}/${BINARY}"

echo ""
echo "Niyantra ${TAG} installed to ${INSTALL_DIR}/${BINARY}"
echo ""
echo "Quick start:"
echo "  niyantra demo    # Try with sample data"
echo "  niyantra serve   # Open http://localhost:9222"
echo "  niyantra snap    # Capture your first Antigravity snapshot"
echo ""
