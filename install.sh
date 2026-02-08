#!/bin/bash
# Ugudu Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/arcslash/ugudu/main/install.sh | bash

set -e

REPO="arcslash/ugudu"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="ugudu"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_banner() {
    echo -e "${BLUE}"
    echo "  _   _                 _       "
    echo " | | | | __ _ _   _  __| |_   _ "
    echo " | | | |/ _\` | | | |/ _\` | | | |"
    echo " | |_| | (_| | |_| | (_| | |_| |"
    echo "  \___/ \__, |\__,_|\__,_|\__,_|"
    echo "        |___/                   "
    echo -e "${NC}"
    echo "AI Team Orchestration"
    echo ""
}

detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$ARCH" in
        x86_64|amd64)
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

    case "$OS" in
        darwin|linux)
            ;;
        mingw*|msys*|cygwin*)
            OS="windows"
            ;;
        *)
            echo -e "${RED}Unsupported OS: $OS${NC}"
            exit 1
            ;;
    esac

    PLATFORM="${OS}_${ARCH}"
    echo -e "${GREEN}Detected platform: $PLATFORM${NC}"
}

get_latest_version() {
    echo -e "${BLUE}Fetching latest version...${NC}"
    VERSION=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/')

    if [ -z "$VERSION" ]; then
        echo -e "${RED}Failed to fetch latest version${NC}"
        exit 1
    fi

    echo -e "${GREEN}Latest version: v${VERSION}${NC}"
}

download_and_install() {
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/v${VERSION}/${BINARY_NAME}_${VERSION}_${PLATFORM}.tar.gz"

    echo -e "${BLUE}Downloading from: $DOWNLOAD_URL${NC}"

    TEMP_DIR=$(mktemp -d)
    trap "rm -rf $TEMP_DIR" EXIT

    curl -fsSL "$DOWNLOAD_URL" | tar xz -C "$TEMP_DIR"

    # Find the binary
    BINARY_PATH=$(find "$TEMP_DIR" -name "$BINARY_NAME" -type f | head -1)

    if [ -z "$BINARY_PATH" ]; then
        echo -e "${RED}Binary not found in archive${NC}"
        exit 1
    fi

    # Install
    echo -e "${BLUE}Installing to ${INSTALL_DIR}...${NC}"

    if [ -w "$INSTALL_DIR" ]; then
        mv "$BINARY_PATH" "${INSTALL_DIR}/${BINARY_NAME}"
        chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    else
        echo -e "${YELLOW}Requires sudo to install to ${INSTALL_DIR}${NC}"
        sudo mv "$BINARY_PATH" "${INSTALL_DIR}/${BINARY_NAME}"
        sudo chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    fi

    echo -e "${GREEN}Successfully installed ${BINARY_NAME} to ${INSTALL_DIR}/${BINARY_NAME}${NC}"
}

verify_installation() {
    if command -v "$BINARY_NAME" &> /dev/null; then
        echo ""
        echo -e "${GREEN}Installation complete!${NC}"
        echo ""
        echo "Run '${BINARY_NAME} --help' to get started."
        echo ""
        echo "Quick start:"
        echo "  1. Start the daemon: ${BINARY_NAME} daemon"
        echo "  2. Open web UI: http://localhost:9741"
        echo ""
    else
        echo -e "${YELLOW}Warning: ${BINARY_NAME} may not be in your PATH${NC}"
        echo "Add ${INSTALL_DIR} to your PATH or run: ${INSTALL_DIR}/${BINARY_NAME}"
    fi
}

main() {
    print_banner
    detect_platform
    get_latest_version
    download_and_install
    verify_installation
}

main "$@"
