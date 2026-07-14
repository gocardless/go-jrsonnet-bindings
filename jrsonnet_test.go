package jrsonnet

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func testdataPath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", name)
}

// --- VM Lifecycle ---

func TestNewAndClose(t *testing.T) {
	vm, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if err := vm.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
}

func TestDoubleClose(t *testing.T) {
	vm, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if err := vm.Close(); err != nil {
		t.Fatalf("first Close() error: %v", err)
	}
	if err := vm.Close(); err != nil {
		t.Fatalf("second Close() error: %v", err)
	}
}

// --- EvaluateSnippet ---

func TestEvaluateSnippetBasic(t *testing.T) {
	vm, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()

	result, err := vm.EvaluateSnippet("test.jsonnet", `{a: 1+1}`)
	if err != nil {
		t.Fatalf("EvaluateSnippet error: %v", err)
	}
	if !strings.Contains(result, `"a": 2`) {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestEvaluateSnippetError(t *testing.T) {
	vm, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()

	_, err = vm.EvaluateSnippet("test.jsonnet", `error "boom"`)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var evalErr *EvaluationError
	if !errors.As(err, &evalErr) {
		t.Fatalf("expected EvaluationError, got %T: %v", err, err)
	}
	if !strings.Contains(evalErr.Error(), "boom") {
		t.Errorf("error should contain 'boom': %s", evalErr.Error())
	}
}

// --- EvaluateFile ---

func TestEvaluateFile(t *testing.T) {
	vm, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()

	result, err := vm.EvaluateFile(testdataPath("simple.jsonnet"))
	if err != nil {
		t.Fatalf("EvaluateFile error: %v", err)
	}
	if !strings.Contains(result, `"greeting": "hello"`) {
		t.Errorf("unexpected result: %s", result)
	}
	if !strings.Contains(result, `"num": 42`) {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestEvaluateFileWithJPath(t *testing.T) {
	vm, err := New(WithJPath(testdataPath("lib")))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()

	result, err := vm.EvaluateFile(testdataPath("with_import.jsonnet"))
	if err != nil {
		t.Fatalf("EvaluateFile error: %v", err)
	}
	if !strings.Contains(result, `"result": "hello world"`) {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestEvaluateFileWithTransitiveImports(t *testing.T) {
	vm, err := New(WithJPath(testdataPath("lib")))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()

	result, err := vm.EvaluateFile(testdataPath("transitive_imports.jsonnet"))
	if err != nil {
		t.Fatalf("EvaluateFile error: %v", err)
	}
	if !strings.Contains(result, `"shouted": "HELLO!"`) {
		t.Errorf("expected shouted=HELLO!, got: %s", result)
	}
	if !strings.Contains(result, `"repeated": "ababab"`) {
		t.Errorf("expected repeated=ababab, got: %s", result)
	}
}

func TestEvaluateFileWithMultipleLibImports(t *testing.T) {
	vm, err := New(WithJPath(testdataPath("lib")))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()

	result, err := vm.EvaluateFile(testdataPath("multi_lib_imports.jsonnet"))
	if err != nil {
		t.Fatalf("EvaluateFile error: %v", err)
	}
	if !strings.Contains(result, `"greeting": "hello world"`) {
		t.Errorf("expected greeting, got: %s", result)
	}
	if !strings.Contains(result, `"shouted": "WORLD!"`) {
		t.Errorf("expected shouted, got: %s", result)
	}
	if !strings.Contains(result, `"doubled": 42`) {
		t.Errorf("expected doubled=42, got: %s", result)
	}
}

func TestEvaluateFileWithDiamondImports(t *testing.T) {
	vm, err := New(WithJPath(testdataPath("lib")))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()

	// diamond_import.jsonnet imports helper, utils, and strings directly.
	// utils also imports strings transitively — tests shared dependency resolution.
	result, err := vm.EvaluateFile(testdataPath("diamond_import.jsonnet"))
	if err != nil {
		t.Fatalf("EvaluateFile error: %v", err)
	}
	if !strings.Contains(result, `"greet": "hello world"`) {
		t.Errorf("expected greet, got: %s", result)
	}
	if !strings.Contains(result, `"shout": "WORLD!"`) {
		t.Errorf("expected shout, got: %s", result)
	}
	if !strings.Contains(result, `"direct_upper": "TEST"`) {
		t.Errorf("expected direct_upper, got: %s", result)
	}
}

// --- Options ---

func TestWithStringOutput(t *testing.T) {
	vm, err := New(WithStringOutput(true))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()

	result, err := vm.EvaluateSnippet("test.jsonnet", `"hello world"`)
	if err != nil {
		t.Fatalf("EvaluateSnippet error: %v", err)
	}
	// With string output and default trailing newline, result is raw string + "\n"
	if result != "hello world\n" {
		t.Errorf("expected 'hello world\\n', got: %q", result)
	}
}

func TestWithTrailingNewline(t *testing.T) {
	t.Run("enabled appends trailing newline (default)", func(t *testing.T) {
		vm, err := New()
		if err != nil {
			t.Fatalf("New() error: %v", err)
		}
		defer vm.Close()

		result, err := vm.EvaluateSnippet("test.jsonnet", `"hello"`)
		if err != nil {
			t.Fatalf("EvaluateSnippet error: %v", err)
		}
		if result != "\"hello\"\n" {
			t.Errorf("expected '\"hello\"\\n', got: %q", result)
		}
	})

	t.Run("false omits trailing newline", func(t *testing.T) {
		vm, err := New(WithTrailingNewline(false))
		if err != nil {
			t.Fatalf("New() error: %v", err)
		}
		defer vm.Close()

		result, err := vm.EvaluateSnippet("test.jsonnet", `"hello"`)
		if err != nil {
			t.Fatalf("EvaluateSnippet error: %v", err)
		}
		if result != "\"hello\"" {
			t.Errorf("expected '\"hello\"' without newline, got: %q", result)
		}
	})

	t.Run("false with EvaluateFile omits trailing newline", func(t *testing.T) {
		vm, err := New(WithTrailingNewline(false))
		if err != nil {
			t.Fatalf("New() error: %v", err)
		}
		defer vm.Close()

		result, err := vm.EvaluateFile(testdataPath("simple.jsonnet"))
		if err != nil {
			t.Fatalf("EvaluateFile error: %v", err)
		}
		if strings.HasSuffix(result, "\n") {
			t.Errorf("expected no trailing newline, got: %q", result)
		}
	})
}

func TestWithMaxStack(t *testing.T) {
	vm, err := New(WithMaxStack(5))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()

	// Recursive function that should overflow with depth=5
	_, err = vm.EvaluateSnippet("test.jsonnet", `
		local f(n) = if n <= 0 then 0 else f(n - 1) + 1;
		f(100)
	`)
	if err == nil {
		t.Fatal("expected stack overflow error")
	}
	if !strings.Contains(err.Error(), "stack") {
		t.Logf("error: %v", err)
	}
}

// --- ExtVar / ExtCode ---

func TestExtVar(t *testing.T) {
	vm, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()

	vm.ExtVar("name", "world")
	result, err := vm.EvaluateSnippet("test.jsonnet", `"hello " + std.extVar("name")`)
	if err != nil {
		t.Fatalf("EvaluateSnippet error: %v", err)
	}
	if !strings.Contains(result, "hello world") {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestExtCode(t *testing.T) {
	vm, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()

	if err := vm.ExtCode("config", `{port: 8080}`); err != nil {
		t.Fatalf("ExtCode error: %v", err)
	}
	result, err := vm.EvaluateSnippet("test.jsonnet", `std.extVar("config").port`)
	if err != nil {
		t.Fatalf("EvaluateSnippet error: %v", err)
	}
	if !strings.Contains(result, "8080") {
		t.Errorf("unexpected result: %s", result)
	}
}

// --- TLAVar / TLACode ---

func TestTLAVar(t *testing.T) {
	vm, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()

	vm.TLAVar("greeting", "hi")
	result, err := vm.EvaluateSnippet("test.jsonnet", `function(greeting) {msg: greeting}`)
	if err != nil {
		t.Fatalf("EvaluateSnippet error: %v", err)
	}
	if !strings.Contains(result, `"msg": "hi"`) {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestTLACode(t *testing.T) {
	vm, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()

	if err := vm.TLACode("x", "10 + 20"); err != nil {
		t.Fatalf("TLACode error: %v", err)
	}
	result, err := vm.EvaluateSnippet("test.jsonnet", `function(x) {val: x}`)
	if err != nil {
		t.Fatalf("EvaluateSnippet error: %v", err)
	}
	if !strings.Contains(result, `"val": 30`) {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestTLAReset(t *testing.T) {
	vm, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()

	vm.TLAVar("x", "first")
	vm.TLAReset()
	vm.TLAVar("x", "second")

	result, err := vm.EvaluateSnippet("test.jsonnet", `function(x) x`)
	if err != nil {
		t.Fatalf("EvaluateSnippet error: %v", err)
	}
	if !strings.Contains(result, "second") {
		t.Errorf("expected 'second', got: %s", result)
	}
}

// --- Custom Importer ---

func TestCustomImporter(t *testing.T) {
	importer := func(base, rel string) (string, []byte, error) {
		if rel == "virtual.jsonnet" {
			return "/virtual/virtual.jsonnet", []byte(`{virtual: true}`), nil
		}
		return "", nil, fmt.Errorf("file not found: %s", rel)
	}

	vm, err := New(WithImporter(importer))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()

	result, err := vm.EvaluateSnippet("test.jsonnet", `import "virtual.jsonnet"`)
	if err != nil {
		t.Fatalf("EvaluateSnippet error: %v", err)
	}
	if !strings.Contains(result, `"virtual": true`) {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestCustomImporterError(t *testing.T) {
	importer := func(base, rel string) (string, []byte, error) {
		return "", nil, fmt.Errorf("custom error: cannot import %s", rel)
	}

	vm, err := New(WithImporter(importer))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()

	_, err = vm.EvaluateSnippet("test.jsonnet", `import "missing.jsonnet"`)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "custom error") {
		t.Errorf("error should mention 'custom error': %v", err)
	}
}

// --- Native Functions ---

func TestNativeFunction(t *testing.T) {
	vm, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()

	vm.RegisterNativeFunction("double", []string{"x"}, func(args []any) (any, error) {
		x, ok := args[0].(float64)
		if !ok {
			return nil, fmt.Errorf("expected number, got %T", args[0])
		}
		return x * 2, nil
	})

	result, err := vm.EvaluateSnippet("test.jsonnet", `std.native("double")(21)`)
	if err != nil {
		t.Fatalf("EvaluateSnippet error: %v", err)
	}
	if !strings.Contains(result, "42") {
		t.Errorf("expected 42, got: %s", result)
	}
}

func TestNativeFunctionStringArgs(t *testing.T) {
	vm, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()

	vm.RegisterNativeFunction("concat", []string{"a", "b"}, func(args []any) (any, error) {
		a, _ := args[0].(string)
		b, _ := args[1].(string)
		return a + b, nil
	})

	result, err := vm.EvaluateSnippet("test.jsonnet", `std.native("concat")("hello", " world")`)
	if err != nil {
		t.Fatalf("EvaluateSnippet error: %v", err)
	}
	if !strings.Contains(result, "hello world") {
		t.Errorf("expected 'hello world', got: %s", result)
	}
}

func TestNativeFunctionError(t *testing.T) {
	vm, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()

	vm.RegisterNativeFunction("fail", []string{}, func(args []any) (any, error) {
		return nil, fmt.Errorf("intentional failure")
	})

	_, err = vm.EvaluateSnippet("test.jsonnet", `std.native("fail")()`)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "intentional failure") {
		t.Errorf("error should contain 'intentional failure': %v", err)
	}
}

// --- Concurrency ---

func TestSequentialEvaluations(t *testing.T) {
	vm, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()

	for i := range 20 {
		snippet := fmt.Sprintf(`{i: %d}`, i)
		result, err := vm.EvaluateSnippet("test.jsonnet", snippet)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		expected := fmt.Sprintf(`"i": %d`, i)
		if !strings.Contains(result, expected) {
			t.Errorf("iteration %d: expected %s in %s", i, expected, result)
		}
	}
}

func TestConcurrentVMs(t *testing.T) {
	var wg sync.WaitGroup
	errs := make(chan error, 10)

	for i := range 10 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			vm, err := New()
			if err != nil {
				errs <- fmt.Errorf("goroutine %d New: %w", idx, err)
				return
			}
			defer vm.Close()

			snippet := fmt.Sprintf(`{goroutine: %d}`, idx)
			result, err := vm.EvaluateSnippet("test.jsonnet", snippet)
			if err != nil {
				errs <- fmt.Errorf("goroutine %d eval: %w", idx, err)
				return
			}
			expected := fmt.Sprintf(`"goroutine": %d`, idx)
			if !strings.Contains(result, expected) {
				errs <- fmt.Errorf("goroutine %d: expected %s in %s", idx, expected, result)
			}
		}(i)
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestConcurrentCallsOnSameVM(t *testing.T) {
	vm, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()

	var wg sync.WaitGroup
	errs := make(chan error, 10)

	for i := range 10 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			snippet := fmt.Sprintf(`{goroutine: %d}`, idx)
			result, err := vm.EvaluateSnippet("test.jsonnet", snippet)
			if err != nil {
				errs <- fmt.Errorf("goroutine %d: %w", idx, err)
				return
			}
			expected := fmt.Sprintf(`"goroutine": %d`, idx)
			if !strings.Contains(result, expected) {
				errs <- fmt.Errorf("goroutine %d: expected %s in %s", idx, expected, result)
			}
		}(i)
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

// --- Stubs ---

func TestStubsReturnNotImplemented(t *testing.T) {
	vm, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()

	if _, err := vm.EvaluateSnippetStream("", ""); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("EvaluateSnippetStream: expected ErrNotImplemented, got %v", err)
	}
	if _, err := vm.EvaluateFileStream(""); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("EvaluateFileStream: expected ErrNotImplemented, got %v", err)
	}
}

// --- EvaluateSnippetMulti / EvaluateFileMulti ---

func TestEvaluateSnippetMultiFields(t *testing.T) {
	vm, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()

	out, err := vm.EvaluateSnippetMulti("multi.jsonnet", `{
  "a.json": {x: 1},
  "b.json": {y: 2},
}`)
	if err != nil {
		t.Fatalf("EvaluateSnippetMulti error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 files, got %d: %v", len(out), out)
	}
	// Each field is manifested independently as JSON, with a trailing newline
	// per file by default (matching go-jsonnet).
	if got, want := out["a.json"], "{\n    \"x\": 1\n}\n"; got != want {
		t.Errorf("a.json = %q, want %q", got, want)
	}
	if got, want := out["b.json"], "{\n    \"y\": 2\n}\n"; got != want {
		t.Errorf("b.json = %q, want %q", got, want)
	}
}

func TestEvaluateFileMulti(t *testing.T) {
	vm, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()

	out, err := vm.EvaluateFileMulti(testdataPath("multi.jsonnet"))
	if err != nil {
		t.Fatalf("EvaluateFileMulti error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 files, got %d: %v", len(out), out)
	}
	if got, want := out["a.json"], "{\n    \"x\": 1\n}\n"; got != want {
		t.Errorf("a.json = %q, want %q", got, want)
	}
}

func TestEvaluateSnippetMultiNonObject(t *testing.T) {
	vm, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()

	if _, err := vm.EvaluateSnippetMulti("multi.jsonnet", `[1, 2, 3]`); err == nil {
		t.Fatal("expected error for non-object top-level value, got nil")
	} else {
		var evalErr *EvaluationError
		if !errors.As(err, &evalErr) {
			t.Errorf("expected *EvaluationError, got %T: %v", err, err)
		}
	}
}

func TestEvaluateSnippetMultiStringOutput(t *testing.T) {
	vm, err := New(WithStringOutput(true))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()

	out, err := vm.EvaluateSnippetMulti("multi.jsonnet", `{
  "greeting.txt": "hello",
  "name.txt": "world",
}`)
	if err != nil {
		t.Fatalf("EvaluateSnippetMulti error: %v", err)
	}
	// String-output mode emits the raw string (not JSON-quoted), plus the
	// per-file trailing newline.
	if got, want := out["greeting.txt"], "hello\n"; got != want {
		t.Errorf("greeting.txt = %q, want %q", got, want)
	}
	if got, want := out["name.txt"], "world\n"; got != want {
		t.Errorf("name.txt = %q, want %q", got, want)
	}
}

func TestEvaluateSnippetMultiNoTrailingNewline(t *testing.T) {
	vm, err := New(WithTrailingNewline(false))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()

	out, err := vm.EvaluateSnippetMulti("multi.jsonnet", `{"a.json": {x: 1}}`)
	if err != nil {
		t.Fatalf("EvaluateSnippetMulti error: %v", err)
	}
	if got, want := out["a.json"], "{\n    \"x\": 1\n}"; got != want {
		t.Errorf("a.json = %q, want %q", got, want)
	}
}

func TestEvaluateSnippetMultiSpecialChars(t *testing.T) {
	vm, err := New(WithStringOutput(true))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()

	// Keys and values with newlines and quotes must round-trip losslessly.
	out, err := vm.EvaluateSnippetMulti("multi.jsonnet", `{
  "weird\nkey\"": "line1\nline2\"quoted\"",
}`)
	if err != nil {
		t.Fatalf("EvaluateSnippetMulti error: %v", err)
	}
	if got, want := out["weird\nkey\""], "line1\nline2\"quoted\"\n"; got != want {
		t.Errorf("special key = %q, want %q", got, want)
	}
}
