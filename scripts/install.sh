#!/bin/sh
# Installs the latest skmgr release for Linux / macOS.
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/AAG1999/skmgr/main/scripts/install.sh | sh
set -e

REPO="AAG1999/skmgr"

echo "Installing skmgr..."

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

if [ "$ARCH" = "x86_64" ]; then
    ARCH="x86_64"
elif [ "$ARCH" = "arm64" ] || [ "$ARCH" = "aarch64" ]; then
    ARCH="arm64"
elif [ "$ARCH" = "i386" ]; then
    ARCH="i386"
else
    echo "Unsupported architecture: $ARCH"
    exit 1
fi

LATEST_RELEASE=$(curl -s https://api.github.com/repos/$REPO/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_RELEASE" ]; then
    echo "Failed to fetch latest release."
    exit 1
fi

case "$OS" in
    darwin*) OS="Darwin" ;;
    linux*) OS="Linux" ;;
    *)
        echo "Unsupported OS: $OS"
        exit 1
        ;;
esac

FILENAME="skmgr_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/$REPO/releases/download/$LATEST_RELEASE/$FILENAME"

echo "Downloading $DOWNLOAD_URL..."
curl -L -o skmgr.tar.gz "$DOWNLOAD_URL"

tar -xzf skmgr.tar.gz skmgr
rm skmgr.tar.gz

chmod +x skmgr

INSTALL_DIR="/usr/local/bin"
if [ ! -w "$INSTALL_DIR" ]; then
    echo "Notice: $INSTALL_DIR is not writable. Falling back to $HOME/.local/bin"
    INSTALL_DIR="$HOME/.local/bin"
    mkdir -p "$INSTALL_DIR"
fi

mv skmgr "$INSTALL_DIR/skmgr"

echo "skmgr installed successfully to $INSTALL_DIR/skmgr"

case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *) echo "Warning: $INSTALL_DIR is not in your PATH. You must add it to run skmgr globally." ;;
esac

"$INSTALL_DIR/skmgr" --version
