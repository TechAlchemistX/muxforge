#!/bin/sh
set -e

GITHUB_REPO="mandeep/muxforge"
BINARY_NAME="muxforge"
INSTALL_DIR="/usr/local/bin"
VERSION="${VERSION:-latest}"

# Detect OS
OS_RAW="$(uname -s)"
case "${OS_RAW}" in
  Linux)
    OS="linux"
    ;;
  Darwin)
    OS="darwin"
    ;;
  *)
    echo "Unsupported operating system: ${OS_RAW}" >&2
    exit 1
    ;;
esac

# Detect architecture
ARCH_RAW="$(uname -m)"
case "${ARCH_RAW}" in
  x86_64)
    ARCH="amd64"
    ;;
  aarch64|arm64)
    ARCH="arm64"
    ;;
  *)
    echo "Unsupported architecture: ${ARCH_RAW}" >&2
    exit 1
    ;;
esac

# Resolve latest version if needed
if [ "${VERSION}" = "latest" ]; then
  VERSION="$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" \
    | grep '"tag_name"' \
    | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')"
fi

echo "-> Installing ${BINARY_NAME} ${VERSION} (${OS}/${ARCH})..."

# Build download URL (goreleaser produces tar.gz archives)
ARCHIVE_NAME="${BINARY_NAME}-${OS}-${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/${ARCHIVE_NAME}"

# Download and extract
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

curl -fsSL "${DOWNLOAD_URL}" -o "${TMP_DIR}/${ARCHIVE_NAME}"
tar -xzf "${TMP_DIR}/${ARCHIVE_NAME}" -C "${TMP_DIR}"
chmod +x "${TMP_DIR}/${BINARY_NAME}"

# Install binary
if [ -w "${INSTALL_DIR}" ]; then
  mv "${TMP_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
else
  sudo mv "${TMP_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
fi

echo "✓ ${BINARY_NAME} installed to ${INSTALL_DIR}/${BINARY_NAME}"
echo "-> Run '${BINARY_NAME} install' to set up your plugins"
