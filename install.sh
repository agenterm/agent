#!/bin/sh
set -e

REPO="agenterm/cli"
BINARY="agenterm"
INSTALL_DIR="/usr/local/bin"

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
    linux)  OS="linux" ;;
    darwin) OS="darwin" ;;
    *)
        echo "Unsupported OS: $OS" >&2
        exit 1
        ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64)  ARCH="amd64" ;;
    aarch64|arm64)  ARCH="arm64" ;;
    *)
        echo "Unsupported architecture: $ARCH" >&2
        exit 1
        ;;
esac

# Get latest version
echo "Fetching latest release..."
VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
if [ -z "$VERSION" ]; then
    echo "Failed to fetch latest version" >&2
    exit 1
fi

FILENAME="${BINARY}-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}"

echo "Downloading ${BINARY} ${VERSION} (${OS}/${ARCH})..."
TMPFILE=$(mktemp)
curl -fSL -o "$TMPFILE" "$URL"
chmod +x "$TMPFILE"

# Install
if [ -w "$INSTALL_DIR" ]; then
    mv "$TMPFILE" "${INSTALL_DIR}/${BINARY}"
else
    echo "Installing to ${INSTALL_DIR} (requires sudo)..."
    sudo mv "$TMPFILE" "${INSTALL_DIR}/${BINARY}"
fi

echo "Installed ${BINARY} ${VERSION} to ${INSTALL_DIR}/${BINARY}"
echo ""
echo "Get started:"
echo "  agenterm init"
echo "  agenterm hook install"
