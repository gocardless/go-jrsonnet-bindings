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
- **cbindgen** (`cargo install cbindgen`) for regenerating the C header

### Build host

The build script (`scripts/build-rust.sh`) unconditionally builds static libraries for all supported targets:

- `aarch64-apple-darwin`
- `aarch64-unknown-linux-gnu`
- `x86_64-unknown-linux-gnu`

It is currently macOS-only: the script relies on Homebrew-installed cross-toolchains and will not run on Linux as-is. On macOS:

```bash
brew tap messense/macos-cross-toolchains
brew install x86_64-unknown-linux-gnu aarch64-unknown-linux-gnu
rustup target install aarch64-apple-darwin x86_64-unknown-linux-gnu aarch64-unknown-linux-gnu
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

Static libraries are placed in `lib/<rust-target-triple>/libjrsonnet_go.a`. The correct one is selected at link time via cgo `LDFLAGS` per `GOOS`/`GOARCH`.

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
