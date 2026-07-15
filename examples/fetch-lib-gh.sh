#!/usr/bin/env bash
set -euo pipefail

# Alternative to fetch-lib.sh that uses the GitHub CLI (`gh`) instead of curl.
# `gh` handles authentication (private repos included) via your existing login
# or GH_TOKEN, and follows the signed asset redirect for you.
#
#   ./fetch-lib-gh.sh <version> <rust-triple> <dest-dir>
#   e.g. ./fetch-lib-gh.sh v1.2.3 x86_64-unknown-linux-gnu .jrsonnet
#
# After extraction the archive lays out:  <dest-dir>/lib/<triple>/libjrsonnet_go.a
# Point the linker at that directory with CGO_LDFLAGS=-L<dest-dir>/lib/<triple>.
#
# This is a generic template. Requires: gh (authenticated), tar,
# sha256sum/shasum.

VERSION="${1:?usage: fetch-lib-gh.sh <version> <triple> <dest-dir>}"
TRIPLE="${2:?usage: fetch-lib-gh.sh <version> <triple> <dest-dir>}"
DEST="${3:?usage: fetch-lib-gh.sh <version> <triple> <dest-dir>}"

REPO="gocardless/go-jrsonnet-bindings"
TARBALL="libjrsonnet_go-${VERSION#v}-${TRIPLE}.tar.gz"

mkdir -p "${DEST}"

# 1. Download the tarball and checksum file for this release.
gh release download "${VERSION}" \
  --repo "${REPO}" \
  --pattern "${TARBALL}" \
  --pattern "SHA256SUMS" \
  --dir "${DEST}" \
  --clobber

# 2. Verify the checksum (portable across coreutils / macOS).
sha_check() { if command -v sha256sum >/dev/null 2>&1; then sha256sum -c -; else shasum -a 256 -c -; fi; }
( cd "${DEST}" && grep " ${TARBALL}\$" SHA256SUMS | sha_check )

# 3. Extract -> ${DEST}/lib/<triple>/libjrsonnet_go.a
tar -xzf "${DEST}/${TARBALL}" -C "${DEST}"
