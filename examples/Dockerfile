# Minimal container build producing a fully static (MUSL) binary suitable for a
# scratch/distroless final image.
#
# This example is MUSL-SPECIFIC. It is only the right approach if you actually
# want a fully static musl binary (e.g. for a scratch/distroless image). If you
# are happy with a dynamically-linked glibc binary, do NOT use this file: build
# from a glibc base image (e.g. golang:1.25 -> debian/gcr base) with a plain
# `make build` (LIBC defaults to gnu) instead.
#
# Note: the musl static build carries a small performance penalty relative to
# the dynamic glibc build. Prefer glibc unless a fully static binary is a hard
# requirement.
#
# Why Alpine for the musl build: a glibc host's musl-gcc is only a wrapper
# around the system gcc, so a "static" link still drags in glibc's libgcc_eh.a
# and fails to resolve against musl. Alpine's toolchain is musl-native end to
# end and links cleanly.

FROM golang:1.25-alpine AS builder

RUN apk add --no-cache bash gcc musl-dev make curl jq git

WORKDIR /src
COPY . .

# make ... LIBC=musl inside this musl-native image:
#   - fetches the *-musl static lib for the build arch,
#   - links fully static with the enlarged stack size (see the Makefile),
#   - CGO_ENABLED=1 is set by the Makefile.
RUN make build LIBC=musl

FROM scratch
COPY --from=builder /src/bin/example /example
ENTRYPOINT ["/example"]
