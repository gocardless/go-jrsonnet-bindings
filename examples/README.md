# Example: consuming go-jrsonnet-bindings

A minimal example of building a program that depends on `go-jrsonnet-bindings`.
The focus is the **build**, not the API — the tricky part of consuming this
library is linking, because the compiled static library is not distributed with
the Go source. Treat everything here as a **template to adapt**, not a turnkey
script: how you access the release artifact (and authenticate, if required),
your target platforms, and your image layout will all differ.

## What's in the box

| File | Purpose |
|---|---|
| `main.go` | Tiny program: create a VM, evaluate one snippet, print it. |
| `fetch-lib.sh` | Download + verify + extract the prebuilt static lib for one target triple (curl + GitHub REST API). |
| `fetch-lib-gh.sh` | Same, but via the `gh` CLI (handles auth for private repos). Interchangeable with `fetch-lib.sh`. |
| `Makefile` | Wire the fetched lib into `go build` via `CGO_LDFLAGS`, incl. musl handling. |
| `Dockerfile` | Fully static (musl) build for a `scratch`/distroless image. |

The `Makefile` fetches libraries into `.jrsonnet/` (the `LIB_ROOT`). These are
downloaded build artifacts, not source — you'll normally want to add `.jrsonnet/`
to your `.gitignore`.

## The build requirements, in short

1. **CGO is mandatory.** The bindings link a Rust staticlib through cgo, so
   every build needs `CGO_ENABLED=1` and a working C toolchain.

2. **The static library is not in git.** The Go module ships source only; the
   `-L${SRCDIR}/lib/...` search paths baked into the package are empty for
   downstream consumers. You must fetch the matching `libjrsonnet_go.a` from a
   release artifact and point the linker at it via
   `CGO_LDFLAGS=-L<dir>` before building.

3. **The library is per-target.** Pick the tarball by your build platform's
   `GOOS`/`GOARCH` and, on Linux, your libc:

   | GOOS | GOARCH | libc | Target triple |
   |------|--------|------|---------------|
   | `darwin` | `arm64` | — | `aarch64-apple-darwin` |
   | `linux` | `amd64` | glibc | `x86_64-unknown-linux-gnu` |
   | `linux` | `arm64` | glibc | `aarch64-unknown-linux-gnu` |
   | `linux` | `amd64` | musl | `x86_64-unknown-linux-musl` |
   | `linux` | `arm64` | musl | `aarch64-unknown-linux-musl` |

   There is no build tag: glibc-vs-musl selection is decided purely by which
   tarball you fetch and link.

4. **musl needs extra linker flags.** For a fully static musl binary you need
   an external linker *and* a larger thread stack — jrsonnet recurses deeply and
   musl's small default stack (~128 KiB) segfaults without it:

   ```
   -ldflags '-linkmode external -extldflags "-static -Wl,-z,stack-size=8388608"'
   ```

   The 8 MiB value here mirrors glibc's default main-thread stack; tune to your
   workload. glibc and darwin builds stay dynamic and need none of this.

   Prefer glibc unless a fully static binary is a hard requirement: the musl
   static build carries a small performance penalty relative to the dynamic
   glibc build.

## Try it

Native (glibc/darwin) build:

```bash
make build
./bin/example      # => { "hello": "world" }
```

Static musl build via Docker (for a scratch/distroless image) — note the
`Dockerfile` is **musl-specific**; for a dynamic glibc container build from a
glibc base image with a plain `make build` instead:

```bash
docker build -t example .
docker run --rm example
```

## Adapting to CI

Because CGO can't cross-compile jrsonnet, build each target **natively** on a
runner of the matching architecture rather than cross-compiling in a single
job (darwin in particular cannot be built on a Linux runner). Set `GOOS`/`GOARCH`
and run `make build` (add `LIBC=musl` for the static variant).
