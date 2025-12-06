#!/bin/bash
# LlamaTerm Installation Script
# https://github.com/adammpkins/llamaterm

set -e

REPO="adammpkins/llamaterm"
BINARY="lt"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${CYAN}"
echo "  _     _                    _____                   "
echo " | |   | |                  |_   _|                  "
echo " | |   | | __ _ _ __ ___   __ | | ___ _ __ _ __ ___  "
echo " | |   | |/ _\` | '_ \` _ \ / _\` | |/ _ \ '__| '_ \` _ \ "
echo " | |___| | (_| | | | | | | (_| | |  __/ |  | | | | | |"
echo " |_____|_|\__,_|_| |_| |_|\__,_\_/\___|_|  |_| |_| |_|"
echo -e "${NC}"
echo ""

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case $ARCH in
    x86_64)
        ARCH="amd64"
        ;;
    arm64|aarch64)
        ARCH="arm64"
        ;;
    *)
        echo -e "${RED}Unsupported architecture: $ARCH${NC}"
        exit 1
        ;;
esac

echo "Detected: ${OS}/${ARCH}"

# Check if Go is installed for building from source
if command -v go &> /dev/null; then
    echo "Go found, building from source..."
    
    # Clone and build
    TEMP_DIR=$(mktemp -d)
    cd "$TEMP_DIR"
    
    git clone --depth 1 https://github.com/${REPO}.git
    cd llamaterm
    
    make build
    
    # Install
    if [ -w "$INSTALL_DIR" ]; then
        cp bin/lt "$INSTALL_DIR/lt"
    else
        echo "Need sudo to install to $INSTALL_DIR"
        sudo cp bin/lt "$INSTALL_DIR/lt"
    fi
    
    # Cleanup
    cd /
    rm -rf "$TEMP_DIR"
else
    echo -e "${RED}Go is not installed. Please install Go first:${NC}"
    echo "  brew install go"
    echo "  # or visit https://go.dev/dl/"
    exit 1
fi

echo ""
echo -e "${GREEN}✓ LlamaTerm installed successfully!${NC}"
echo ""
echo "Get started:"
echo "  lt ask \"Hello, world!\""
echo "  lt cmd \"list files\""
echo "  lt chat"
echo ""
echo "Configure:"
echo "  lt config init"
echo ""

# Suggest adding to PATH if needed
if ! command -v lt &> /dev/null; then
    echo -e "${CYAN}Note: Make sure $INSTALL_DIR is in your PATH${NC}"
fi
