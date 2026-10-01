package jrsonnet

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestNativeFunctionPanic(t *testing.T) {
	vm, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()
	vm.RegisterNativeFunction("f", []string{}, func([]any) (any, error) { panic("boom") })

	_, err = vm.EvaluateSnippet("t.jsonnet", `std.native("f")()`)
	if err == nil || !strings.Contains(err.Error(), `native function "f" panicked: boom`) {
		t.Errorf("got %v, want the panic as an error", err)
	}
	// The VM is still usable.
	if out, err := vm.EvaluateSnippet("t.jsonnet", `1`); err != nil || out != "1\n" {
		t.Errorf("after panic: got %q, %v", out, err)
	}
}

// jrsonnet's manifest encodes the arguments, replacing val_to_json.
func TestNativeFunctionArgumentEncoding(t *testing.T) {
	vm, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()
	var got []any
	vm.RegisterNativeFunction("f", []string{"x", "y"}, func(args []any) (any, error) {
		got = args
		return nil, nil
	})

	if _, err := vm.EvaluateSnippet("t.jsonnet", `std.native("f")({a: [1e20, 0.5, null, "é"], h:: 1}, true)`); err != nil {
		t.Fatalf("EvaluateSnippet error: %v", err)
	}
	// The hidden field is left out.
	want := []any{map[string]any{"a": []any{1e20, 0.5, nil, "é"}}, true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("args = %#v, want %#v", got, want)
	}
}

// jrsonnet's serde support decodes the result, replacing json_to_val.
func TestNativeFunctionResultDecoding(t *testing.T) {
	vm, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()
	vm.RegisterNativeFunction("f", []string{}, func([]any) (any, error) {
		return map[string]any{"a": []any{1e20, 0.5, nil, "é"}, "b": true}, nil
	})

	out, err := vm.EvaluateSnippet("t.jsonnet", `std.native("f")() == {a: [1e20, 0.5, null, "é"], b: true}`)
	if err != nil || out != "true\n" {
		t.Errorf("got %q, %v; want true", out, err)
	}
}

// As in go-jsonnet, an argument that can't be encoded fails the call, and the
// function isn't called.
func TestNativeFunctionFailingArguments(t *testing.T) {
	vm, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()
	called := false
	vm.RegisterNativeFunction("f", []string{"x"}, func([]any) (any, error) {
		called = true
		return nil, nil
	})

	for _, arg := range []string{`[1, error "boom"]`, `{a: error "boom"}`, `function(x) x`} {
		called = false
		if _, err := vm.EvaluateSnippet("t.jsonnet", `std.native("f")(`+arg+`)`); err == nil {
			t.Errorf("argument %s: want an error", arg)
		}
		if called {
			t.Errorf("argument %s: the function was called", arg)
		}
	}
}

func TestRegisterNativeFunctionConcurrently(t *testing.T) {
	vm, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer vm.Close()
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			vm.RegisterNativeFunction(fmt.Sprint("f", i), []string{}, func([]any) (any, error) { return i, nil })
		}()
	}
	wg.Wait()

	out, err := vm.EvaluateSnippet("t.jsonnet", `std.native("f3")()`)
	if err != nil || out != "3\n" {
		t.Errorf("got %q, %v; want 3", out, err)
	}
}

func TestRegisterNativeFunctionAfterClose(t *testing.T) {
	vm, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	_ = vm.Close()
	vm.RegisterNativeFunction("f", []string{}, func([]any) (any, error) { return nil, nil })
	if len(vm.nativeHandles) != 0 {
		t.Errorf("handle created for a closed VM")
	}
}
