#!/usr/bin/env bash
set -euo pipefail

# Fetch the prebuilt jrsonnet static library for a given release + target triple
# and extract it locally, so a downstream build can link against it.
#
#   ./fetch-lib.sh <version> <rust-triple> <dest-dir>
#   e.g. ./fetch-lib.sh v1.2.3 x86_64-unknown-linux-gnu .jrsonnet
#
# After extraction the archive lays out:  <dest-dir>/lib/<triple>/libjrsonnet_go.a
# Point the linker at that directory with CGO_LDFLAGS=-L<dest-dir>/lib/<triple>.
#
# This is a generic template, so adapt it to your build. It sends no
# credentials, and GitHub rate limits unauthenticated API requests per IP
# address. If your CI hits that limit, add an Authorization header to the curl
# calls or use fetch-lib-gh.sh. Requires: curl, jq, tar, sha256sum/shasum.

VERSION="${1:?usage: fetch-lib.sh <version> <triple> <dest-dir>}"
TRIPLE="${2:?usage: fetch-lib.sh <version> <triple> <dest-dir>}"
DEST="${3:?usage: fetch-lib.sh <version> <triple> <dest-dir>}"

REPO="gocardless/go-jrsonnet-bindings"
API="https://api.github.com/repos/${REPO}"
TARBALL="libjrsonnet_go-${VERSION#v}-${TRIPLE}.tar.gz"

mkdir -p "${DEST}"

# 1. Resolve asset ids for this tag.
release="$(curl -fsSL -H "Accept: application/vnd.github+json" \
  "${API}/releases/tags/${VERSION}")"
asset_id() { jq -r --arg n "$1" '.assets[] | select(.name==$n) | .id' <<<"${release}"; }

tar_id="$(asset_id "${TARBALL}")"
[[ -n "${tar_id}" && "${tar_id}" != null ]] || {
  echo "${TARBALL} not found in release ${VERSION}" >&2; exit 1; }

# 2. Download by asset id; -L follows the signed storage redirect.
dl() { curl -fSL -H "Accept: application/octet-stream" \
  "${API}/releases/assets/$1" -o "$2"; }

dl "${tar_id}" "${DEST}/${TARBALL}"
dl "$(asset_id "SHA256SUMS")" "${DEST}/SHA256SUMS"

# 3. Verify the checksum (portable across coreutils / macOS).
sha_check() { if command -v sha256sum >/dev/null 2>&1; then sha256sum -c -; else shasum -a 256 -c -; fi; }
( cd "${DEST}" && grep " ${TARBALL}\$" SHA256SUMS | sha_check )

# 4. Extract -> ${DEST}/lib/<triple>/libjrsonnet_go.a
tar -xzf "${DEST}/${TARBALL}" -C "${DEST}"
