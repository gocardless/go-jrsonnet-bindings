#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Cross Compiler Setup
# brew tap messense/macos-cross-toolchains
# brew install x86_64-unknown-gnu x86_64-linux-gnu-binutils
export PATH="${PATH}:$(brew --prefix x86_64-unknown-linux-gnu)/bin"

CARGO="cargo"
# # Use rustup toolchain if available, otherwise default cargo
# if command -v rustup &>/dev/null; then
#     CARGO="rustup run stable cargo"
# else
#     CARGO="cargo"
# fi

TARGETS=(
    "aarch64-apple-darwin"
    "x86_64-unknown-linux-gnu"
    # wasm32-unknown-unknown
)
for TARGET in "${TARGETS[@]}"; do
    echo "Building for ${TARGET}..."
    cd "${ROOT_DIR}"
    mkdir -p "lib/${TARGET}"

    $CARGO build --release --lib --target="${TARGET}"

    cp "${ROOT_DIR}/target/${TARGET}/release/libjrsonnet_go.a" "lib/${TARGET}/"
    echo -e "\n\nStatic library built at:\n\tlib/${TARGET}/libjrsonnet_go.a"
done
