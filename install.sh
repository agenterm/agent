#!/bin/sh
set -e

REPO="agenterm/cli"
BINARY="agenterm"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

TMPDIR_CLEANUP=""
cleanup() {
    [ -n "$TMPDIR_CLEANUP" ] && rm -rf "$TMPDIR_CLEANUP"
}
trap cleanup EXIT INT TERM

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
    aarch64|arm64) ARCH="arm64" ;;
    *)
        echo "Unsupported architecture: $ARCH" >&2
        exit 1
        ;;
esac

# Require curl
if ! command -v curl >/dev/null 2>&1; then
    echo "Error: curl is required but not installed." >&2
    exit 1
fi

# Get latest version
echo "Fetching latest release..."
VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | awk -F'"' '/"tag_name"/{print $4}')
if [ -z "$VERSION" ]; then
    echo "Failed to fetch latest version" >&2
    exit 1
fi

FILENAME="${BINARY}-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}"
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"

TMPDIR_CLEANUP=$(mktemp -d)
TMPFILE="${TMPDIR_CLEANUP}/${FILENAME}"
CHECKSUMS_FILE="${TMPDIR_CLEANUP}/checksums.txt"

echo "Downloading ${BINARY} ${VERSION} (${OS}/${ARCH})..."
curl -fsSL -o "$TMPFILE" "$URL" &
pid1=$!
curl -fsSL -o "$CHECKSUMS_FILE" "$CHECKSUMS_URL" &
pid2=$!
wait "$pid1"
wait "$pid2"

# Verify checksum
echo "Verifying checksum..."
if command -v sha256sum >/dev/null 2>&1; then
    ACTUAL=$(sha256sum "$TMPFILE" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
    ACTUAL=$(shasum -a 256 "$TMPFILE" | awk '{print $1}')
else
    echo "Warning: no sha256sum or shasum found, skipping checksum verification" >&2
    ACTUAL=""
fi

if [ -n "$ACTUAL" ]; then
    EXPECTED=$(grep -F "$FILENAME" "$CHECKSUMS_FILE" | awk '{print $1}')
    if [ -z "$EXPECTED" ]; then
        echo "Error: checksum for ${FILENAME} not found in checksums.txt" >&2
        exit 1
    fi
    if [ "$ACTUAL" != "$EXPECTED" ]; then
        echo "Error: checksum mismatch" >&2
        echo "  expected: ${EXPECTED}" >&2
        echo "  actual:   ${ACTUAL}" >&2
        exit 1
    fi
    echo "Checksum verified."
fi

chmod +x "$TMPFILE"

# Install
if ! mv "$TMPFILE" "${INSTALL_DIR}/${BINARY}" 2>/dev/null; then
    echo "Installing to ${INSTALL_DIR} (requires sudo)..."
    sudo mv "$TMPFILE" "${INSTALL_DIR}/${BINARY}"
fi

echo "Installed ${BINARY} ${VERSION} to ${INSTALL_DIR}/${BINARY}"
echo ""
echo "Get started:"
echo "  agenterm init"
echo "  agenterm hook install"
