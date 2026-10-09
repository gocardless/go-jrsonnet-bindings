#!/usr/bin/env bash
# Build the libjrsonnet_go.a static library into lib/<target-triple>/.
#
# Usage: scripts/build-rust.sh [native|all|<target-triple>...]
#
#   native (default)  build only for the host's Rust target triple
#   all               build every target in the host OS's target list (below)
#   <target-triple>   build the given triple(s)
#
# The toolchain environment (rustup targets, cross linkers, musl-gcc, ...) is the
# caller's responsibility. On macOS, the Homebrew cross toolchains below are added
# to PATH automatically when installed. Linkers and C compilers can be overridden
# through the usual cargo/cc-rs variables (CARGO_TARGET_<TRIPLE>_LINKER, CC_<triple>).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ROOT_DIR}"

CARGO="${CARGO:-cargo}"

usage() {
    sed -n '2,/^set -euo/{/^set -euo/d;s/^# \{0,1\}//;p;}' "${BASH_SOURCE[0]}"
}

# Targets built by `all`, per host OS.
case "$(uname -s)" in
    Darwin)
        ALL_TARGETS=(
            "aarch64-apple-darwin"
            "aarch64-unknown-linux-gnu"
            "x86_64-unknown-linux-gnu"
            "aarch64-unknown-linux-musl"
            "x86_64-unknown-linux-musl"
            # wasm32-unknown-unknown
        )
        ;;
    Linux)
        ALL_TARGETS=(
            "aarch64-unknown-linux-gnu"
            "x86_64-unknown-linux-gnu"
            "aarch64-unknown-linux-musl"
            "x86_64-unknown-linux-musl"
        )
        ;;
    *)
        echo "error: unsupported host OS: $(uname -s)" >&2
        exit 1
        ;;
esac

# Run from ROOT_DIR so rust-toolchain.toml selects the toolchain.
native_target() {
    rustc -vV | sed -n 's/^host: //p'
}

# Cross compiler setup (macOS host -> Linux targets):
#   brew tap messense/macos-cross-toolchains
#   brew install x86_64-unknown-linux-gnu aarch64-unknown-linux-gnu
#   brew install x86_64-unknown-linux-musl aarch64-unknown-linux-musl
setup_macos_cross() {
    local triple var
    for triple in \
        x86_64-unknown-linux-gnu aarch64-unknown-linux-gnu \
        x86_64-unknown-linux-musl aarch64-unknown-linux-musl; do
        if command -v brew >/dev/null 2>&1; then
            local prefix
            prefix="$(brew --prefix "${triple}" 2>/dev/null)" || true
            if [[ -d "${prefix}/bin" ]]; then
                export PATH="${PATH}:${prefix}/bin"
            fi
        fi
        var="CARGO_TARGET_$(echo "${triple}" | tr '[:lower:]-' '[:upper:]_')_LINKER"
        export "${var}=${!var:-${triple}-gcc}"
    done
}

case "${1:-native}" in
    -h | --help)
        usage
        exit 0
        ;;
    native)
        TARGETS=("$(native_target)")
        ;;
    all)
        TARGETS=("${ALL_TARGETS[@]}")
        ;;
    *)
        TARGETS=("$@")
        ;;
esac

if [[ "$(uname -s)" == Darwin ]]; then
    setup_macos_cross
fi

for TARGET in "${TARGETS[@]}"; do
    echo "Building for ${TARGET}..."
    $CARGO build --release --lib --target="${TARGET}"

    mkdir -p "lib/${TARGET}"

    cp "target/${TARGET}/release/libjrsonnet_go.a" "lib/${TARGET}/"
    echo -e "\n\nStatic library built at:\n\tlib/${TARGET}/libjrsonnet_go.a"
done
