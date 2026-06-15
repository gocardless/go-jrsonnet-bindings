# go-jrsonnet-bindings

Go bindings for [jrsonnet](https://github.com/CertainLach/jrsonnet), a Rust implementation of the [Jsonnet](https://jsonnet.org/) data templating language.

## Overview

This package provides a Go-native API for jrsonnet via CGo. Instead of using jrsonnet's existing C bindings, a custom Rust staticlib wraps jrsonnet-evaluator's Rust API directly and exposes it through a C ABI tailored for Go interop.

### Features

- Evaluate Jsonnet snippets and files
- External variables (`std.extVar`) and top-level arguments
- Custom import callbacks for virtual filesystems
- Native extension functions via `std.native`
- Thread-safe: a single VM can be shared across goroutines
- JSON-based native callback marshalling (no raw pointer management)
- Rust panics caught at the FFI boundary (never crashes the Go process)

## Prerequisites

- **Go** 1.25+
- **Rust** stable toolchain (for building the staticlib)
- **cbindgen** (`cargo install cbindgen`) — only needed to regenerate the C header after changing the Rust FFI API; not required for normal builds or running tests

### Build host

The build script (`scripts/build-rust.sh`) unconditionally builds static libraries for all supported targets:

- `aarch64-apple-darwin`
- `aarch64-unknown-linux-gnu`
- `aarch64-unknown-linux-musl`
- `x86_64-unknown-linux-gnu`
- `x86_64-unknown-linux-musl`

It is currently macOS-only: the script relies on Homebrew-installed cross-toolchains and will not run on Linux as-is. On macOS:

```bash
brew tap messense/macos-cross-toolchains
brew install x86_64-unknown-linux-gnu aarch64-unknown-linux-gnu
brew install x86_64-unknown-linux-musl aarch64-unknown-linux-musl
rustup target install \
  aarch64-apple-darwin \
  x86_64-unknown-linux-gnu \
  aarch64-unknown-linux-gnu \
  x86_64-unknown-linux-musl \
  aarch64-unknown-linux-musl
```

## Building

```bash
# Build the Rust staticlibs for all supported targets
make rust

# Regenerate the C header (only needed after Rust API changes)
make header

# Run tests
make test
```

Static libraries are placed in `lib/<rust-target-triple>/libjrsonnet_go.a`, one per
target triple:

| GOOS | GOARCH | libc | Target triple |
|------|--------|------|------------------------------|
| `darwin` | `arm64` | — | `aarch64-apple-darwin` |
| `linux` | `arm64` | glibc | `aarch64-unknown-linux-gnu` |
| `linux` | `amd64` | glibc | `x86_64-unknown-linux-gnu` |
| `linux` | `arm64` | musl | `aarch64-unknown-linux-musl` |
| `linux` | `amd64` | musl | `x86_64-unknown-linux-musl` |

## Distribution

**The static libraries are not tracked in git** (`lib/` is gitignored). They are
built in CI and published as release tarballs — one per target triple — attached to
each GitHub release (see `.github/workflows/build.yaml`). The Go module itself ships
only source; it contains no `.a` files.

Because of this, the cgo `-L${SRCDIR}/lib/<triple>` search paths baked into
`jrsonnet.go` are empty for downstream consumers. They resolve only when building
this repo directly (e.g. `make test`, after `make rust` populates `lib/`). A consumer
**must fetch the matching library before building** and point the linker at it.

## Consuming this library

1. Fetch the release tarball for your target triple and extract it so that
   `<dir>/libjrsonnet_go.a` exists. The triple is chosen by your build platform and
   libc — glibc for ordinary Linux/macOS builds, musl for Alpine/`scratch` images.
2. Build with cgo enabled, pointing `CGO_LDFLAGS` at that directory:

```bash
CGO_ENABLED=1 CGO_LDFLAGS="-L/path/to/lib/<triple>" go build ./...
```

There is **no build tag**: the linker resolves `-ljrsonnet_go` from the directory you
supply via `CGO_LDFLAGS`, so glibc-vs-musl selection is determined entirely by which
tarball you fetch. For a worked example — fetching glibc libs for native builds and
staging musl libs inside an Alpine container build — see the consumer Makefile and
`Dockerfile.alpine-test` in the `utopia` repository.

## Usage

```go
package main

import (
    "fmt"
    "log"

    jrsonnet "github.com/gocardless/go-jrsonnet-bindings"
)

func main() {
    vm, err := jrsonnet.New(
        jrsonnet.WithMaxStack(500),
        jrsonnet.WithJPath("./lib"),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer vm.Close()

    // External variables
    vm.ExtVar("name", "world")

    // Evaluate a snippet
    result, err := vm.EvaluateSnippet("example.jsonnet", `{
        greeting: "hello " + std.extVar("name"),
    }`)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(result)
}
```

### Custom Import Callback

```go
vm, _ := jrsonnet.New(jrsonnet.WithImporter(
    func(base, rel string) (string, []byte, error) {
        if rel == "config.jsonnet" {
            return "/virtual/config.jsonnet", []byte(`{port: 8080}`), nil
        }
        return "", nil, fmt.Errorf("file not found: %s", rel)
    },
))
defer vm.Close()

result, _ := vm.EvaluateSnippet("main.jsonnet", `import "config.jsonnet"`)
```

### Native Extension Functions

```go
vm, _ := jrsonnet.New()
defer vm.Close()

vm.RegisterNativeFunction("double", []string{"x"}, func(args []any) (any, error) {
    x, _ := args[0].(float64)
    return x * 2, nil
})

result, _ := vm.EvaluateSnippet("test.jsonnet", `std.native("double")(21)`)
// result: "42\n"
```

### Top-Level Arguments

```go
vm, _ := jrsonnet.New()
defer vm.Close()

vm.TLAVar("env", "production")
vm.TLACode("port", "8080")

result, _ := vm.EvaluateSnippet("test.jsonnet", `function(env, port) {
    environment: env,
    port: port,
}`)
```

## API Reference

### VM Lifecycle

| Function | Description |
|---|---|
| `New(opts ...Option) (*VM, error)` | Create a new Jsonnet VM |
| `(*VM).Close() error` | Destroy the VM and free resources |

### Evaluation

| Method | Description |
|---|---|
| `EvaluateSnippet(filename, snippet string) (string, error)` | Evaluate a Jsonnet code string |
| `EvaluateFile(filename string) (string, error)` | Evaluate a Jsonnet file |

> **Note:** `EvaluateSnippetMulti`, `EvaluateFileMulti`, `EvaluateSnippetStream`, and `EvaluateFileStream` are defined but not yet implemented — they return `ErrNotImplemented`.

### Options (passed to `New`)

| Option | Description |
|---|---|
| `WithMaxStack(depth uint)` | Set maximum stack depth |
| `WithMaxTrace(lines uint)` | Set maximum stack trace lines (0 = all) |
| `WithStringOutput(enabled bool)` | Output raw strings instead of JSON |
| `WithJPath(paths ...string)` | Add library search paths |
| `WithImporter(fn ImportFunc)` | Set a custom import callback |

### Variables

| Method | Description |
|---|---|
| `ExtVar(key, val string)` | Bind an external string variable |
| `ExtCode(key, code string) error` | Bind an external code variable |
| `TLAVar(key, val string)` | Bind a top-level string argument |
| `TLACode(key, code string) error` | Bind a top-level code argument |
| `TLAReset()` | Clear all TLA arguments |

### Native Functions

| Method | Description |
|---|---|
| `RegisterNativeFunction(name string, params []string, fn NativeFunc)` | Register a native extension function |

### Error Types

| Type | Description |
|---|---|
| `EvaluationError` | Jsonnet evaluation error |
| `PanicError` | Rust panic caught at FFI boundary |
| `ErrVMClosed` | Operation on a closed VM |
| `ErrNotImplemented` | Stub for unimplemented features |

## Thread Safety

A `VM` is safe to share across goroutines: calls are serialised internally and run in FIFO order.

Things to be aware of:

- Each `VM` owns one dedicated OS thread for its entire lifetime, so prefer reusing a long-lived VM over creating a new one per request.
- Only one evaluation runs at a time per VM. For parallelism, create multiple VMs.
- `Close()` waits for any in-flight call to finish before tearing the VM down.

## Architecture

```
Go (CGo)  ──>  C ABI  ──>  Rust staticlib  ──>  jrsonnet-evaluator
                                │
              ┌─────────────────┴────────────────────────┐
              │  Custom Rust FFI layer                   │
              │  (crates/jrsonnet_go/src/)               │
              │  - VM lifecycle + GC                     │
              │  - GoResult error codes                  │
              │  - JSON-based callback marshalling       │
              └──────────────────────────────────────────┘
```

Callbacks (import and native functions) flow in reverse: Rust calls a C function pointer, which bridges to a Go export function, which looks up the registered Go callback via `cgo.Handle`.

## License

See upstream [jrsonnet](https://github.com/CertainLach/jrsonnet) for the evaluator license.
