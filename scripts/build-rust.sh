#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Cross compiler setup (macOS host -> Linux targets):
#   brew tap messense/macos-cross-toolchains
#   brew install x86_64-unknown-linux-gnu aarch64-unknown-linux-gnu
#   brew install x86_64-unknown-linux-musl aarch64-unknown-linux-musl
export PATH="${PATH}:$(brew --prefix x86_64-unknown-linux-gnu)/bin"
export PATH="${PATH}:$(brew --prefix aarch64-unknown-linux-gnu)/bin"
export PATH="${PATH}:$(brew --prefix x86_64-unknown-linux-musl)/bin"
export PATH="${PATH}:$(brew --prefix aarch64-unknown-linux-musl)/bin"
export CARGO_TARGET_X86_64_UNKNOWN_LINUX_GNU_LINKER="x86_64-unknown-linux-gnu-gcc"
export CARGO_TARGET_AARCH64_UNKNOWN_LINUX_GNU_LINKER="aarch64-unknown-linux-gnu-gcc"
export CARGO_TARGET_X86_64_UNKNOWN_LINUX_MUSL_LINKER="x86_64-unknown-linux-musl-gcc"
export CARGO_TARGET_AARCH64_UNKNOWN_LINUX_MUSL_LINKER="aarch64-unknown-linux-musl-gcc"

CARGO="cargo"
# # Use rustup toolchain if available, otherwise default cargo
# if command -v rustup &>/dev/null; then
#     CARGO="rustup run stable cargo"
# else
#     CARGO="cargo"
# fi

TARGETS=(
    "aarch64-apple-darwin"
    "aarch64-unknown-linux-gnu"
    "x86_64-unknown-linux-gnu"
    "aarch64-unknown-linux-musl"
    "x86_64-unknown-linux-musl"
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
