#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
RUST_DIR="${ROOT_DIR}/rust"

CARGO="cargo"
# # Use rustup toolchain if available, otherwise default cargo
# if command -v rustup &>/dev/null; then
#     CARGO="rustup run stable cargo"
# else
#     CARGO="cargo"
# fi

# Detect platform
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "${OS}" in
    darwin) PLATFORM_OS="darwin" ;;
    linux)  PLATFORM_OS="linux" ;;
    *)      echo "Unsupported OS: ${OS}"; exit 1 ;;
esac

case "${ARCH}" in
    arm64|aarch64) PLATFORM_ARCH="arm64" ;;
    x86_64|amd64)  PLATFORM_ARCH="amd64" ;;
    *)              echo "Unsupported architecture: ${ARCH}"; exit 1 ;;
esac

PLATFORM="${PLATFORM_OS}-${PLATFORM_ARCH}"
LIB_DIR="${ROOT_DIR}/lib/${PLATFORM}"

echo "Building for ${PLATFORM}..."
mkdir -p "${LIB_DIR}"

cd "${RUST_DIR}"
$CARGO build --release

cp "${RUST_DIR}/target/release/libjrsonnet_go.a" "${LIB_DIR}/"

echo "Static library built at ${LIB_DIR}/libjrsonnet_go.a"
