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

`scripts/build-rust.sh` (`make rust`) builds the static library for the host's native target only. `scripts/build-rust.sh all` (`make rust-all`) builds every supported target for the host OS — on macOS:

- `aarch64-apple-darwin`
- `aarch64-unknown-linux-gnu`
- `aarch64-unknown-linux-musl`
- `x86_64-unknown-linux-gnu`
- `x86_64-unknown-linux-musl`

On Linux, `all` builds the four Linux targets. Explicit target triples can also be passed (`scripts/build-rust.sh x86_64-unknown-linux-musl`).

Setting up the toolchains (rustup targets, cross C compilers/linkers, `musl-gcc`) is up to you; the script only uses what is on `PATH`, plus the usual `CARGO_TARGET_<TRIPLE>_LINKER` / `CC_<triple>` overrides. On macOS, the Homebrew cross-toolchains below are picked up automatically:

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
# Build the Rust staticlib for the host target
make rust

# Build the Rust staticlibs for all supported targets
make rust-all

# Regenerate the C header (only needed after Rust API changes)
make header

# Run tests, including the comparison with go-jsonnet
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

The essentials:

- Build with **CGO enabled** (`CGO_ENABLED=1`) — the bindings link a Rust staticlib.
- **Fetch the matching library first.** Download the release tarball for your target
  triple, extract it so `<dir>/libjrsonnet_go.a` exists, and point the linker at it:
  `CGO_LDFLAGS="-L<dir>"`. There is no build tag — glibc-vs-musl is decided purely by
  which tarball you fetch.
- **musl needs extra flags** (`-linkmode external -extldflags "-static -Wl,-z,stack-size=8388608"`);
  glibc/darwin builds stay dynamic. Prefer glibc unless a fully static binary is a hard
  requirement — musl carries a small performance penalty.

See [`examples/`](./examples) for a minimal end-to-end consumer (program, fetch script,
`Makefile`, and musl `Dockerfile`) with the details.

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
| `EvaluateSnippetMulti(filename, snippet string) (map[string]string, error)` | Evaluate a snippet in multi mode (top-level object → file name to output) |
| `EvaluateFileMulti(filename string) (map[string]string, error)` | Evaluate a file in multi mode |
| `EvaluateSnippetStream(filename, snippet string) ([]string, error)` | Evaluate a snippet in stream mode (top-level array → ordered outputs) |
| `EvaluateFileStream(filename string) ([]string, error)` | Evaluate a file in stream mode |

### Options (passed to `New`)

| Option | Description |
|---|---|
| `WithMaxStack(depth uint)` | Set maximum stack depth (default 500) |
| `WithMaxTrace(frames uint)` | Set maximum stack frames shown in an error (default 20, 0 = all) |
| `WithStringOutput(enabled bool)` | Output raw strings instead of JSON (the result must be a string) |
| `WithTrailingNewline(enabled bool)` | Append a newline to each output (default true) |
| `WithJPath(paths ...string)` | Add library search paths (a later path wins) |
| `WithImporter(fn ImportFunc)` | Set a custom import callback (can't be combined with `WithJPath`) |
| `WithGCInterval(n uint)` | Collect reference cycles after every `n` evaluations (default 0 = only on `Close`); see below |

jrsonnet never collects reference cycles (created by objects, for example) by
itself, so a VM evaluated many times keeps growing until `Close`.
`WithGCInterval` collects them every `n` evaluations.

### Variables

| Method | Description |
|---|---|
| `ExtVar(key, val string)` | Bind an external string variable |
| `ExtCode(key, code string)` | Bind an external code variable |
| `TLAVar(key, val string)` | Bind a top-level string argument |
| `TLACode(key, code string)` | Bind a top-level code argument |
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

## Differences from go-jsonnet

The [`conformance`](./conformance) module runs the same cases through this
package and [go-jsonnet](https://github.com/google/go-jsonnet) v0.22.0, and
compares the results. It is a separate Go module, so this package doesn't
depend on go-jsonnet. Each difference below has a case there.

Error messages differ in wording, and the tests don't compare them.

- `std.thisFile` in `EvaluateSnippet` is `"virtual:<filename>"`. In
  go-jsonnet's `EvaluateAnonymousSnippet` it is `""`.
- `std.thisFile` in a file resolves symlinks, and is relative to the working
  directory at `New()` unless that would climb to the root. Error messages
  show absolute paths with symlinks resolved. go-jsonnet shows paths as given.
- The value of a file passed to `EvaluateFile` is cached like an imported
  file's, so evaluating it again reuses it. go-jsonnet evaluates the file
  again, so side effects such as `std.trace` and native function calls run
  again too.
- Importing a file that isn't valid UTF-8 with `import` or `importstr` is an
  error, as in jrsonnet. go-jsonnet replaces each invalid byte with U+FFFD.
- A native function may return anything `encoding/json` can marshal, such as
  `[]string`. go-jsonnet only accepts the types it passes in.
- Stream mode honours string output and the trailing newline setting, like
  the other modes and jrsonnet's own C bindings. go-jsonnet ignores both in
  stream mode, and always outputs JSON followed by a newline.

## Thread Safety

A `VM` is safe to share across goroutines. It runs calls one at a time, in no guaranteed order.

Things to be aware of:

- Each `VM` owns one dedicated OS thread for its entire lifetime, so prefer reusing a long-lived VM over creating a new one per request.
- Only one evaluation runs at a time per VM. For parallelism, create multiple VMs.
- `Close()` waits for any in-flight call to finish before tearing the VM down.
- Native functions and importers run on the VM's thread during an evaluation. They must not call the VM, which deadlocks, nor `runtime.Goexit`.
- Past a few thousand stack frames, jrsonnet continues on a stack it allocates itself. A native function or importer called from there crashes the process, so keep `WithMaxStack` well below that when using either.

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
