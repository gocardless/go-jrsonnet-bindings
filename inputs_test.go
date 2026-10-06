package jrsonnet

import (
	"encoding/json"
	"strings"
	"testing"
)

func newTestVM(t testing.TB, opts ...Option) *VM {
	t.Helper()
	vm, err := New(opts...)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	t.Cleanup(func() { _ = vm.Close() })
	return vm
}

// extVarRoundTrip binds s as an ext var and returns what Jsonnet sees.
func extVarRoundTrip(t testing.TB, vm *VM, s string) string {
	t.Helper()
	vm.ExtVar("k", s)
	out, err := vm.EvaluateSnippet("t.jsonnet", `std.extVar("k")`)
	if err != nil {
		t.Fatalf("EvaluateSnippet error: %v", err)
	}
	var got string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	return got
}

// FuzzInputDecoding checks that strings are decoded like Go's
// string([]rune(s)), which is how go-jsonnet sees them.
func FuzzInputDecoding(f *testing.F) {
	for _, s := range []string{
		"", "plain", "héllo ☃ 𝄞", "a\x00b", "\xff\xfe", "\xe2\x82A",
		"a\xffb\xe2\x82c", "\xf0\x9f\x98", "\xed\xa0\x80", "\xc0\x80",
	} {
		f.Add(s)
	}
	vm := newTestVM(f)
	f.Fuzz(func(t *testing.T, s string) {
		if got, want := extVarRoundTrip(t, vm, s), string([]rune(s)); got != want {
			t.Errorf("ExtVar(%q): got %q, want %q", s, got, want)
		}
	})
}

func TestInvalidUTF8Inputs(t *testing.T) {
	vm := newTestVM(t)
	vm.ExtVar("\xff", "ignored")
	vm.ExtCode("\xfe", "ignored")
	vm.RegisterNativeFunction("\xfd", nil, func([]any) (any, error) { return nil, nil })
	vm.ExtCode("code", "'\xff'")

	out, err := vm.EvaluateSnippet("\xff.jsonnet", "[std.extVar('code'), '\xfe']")
	if err != nil {
		t.Fatalf("EvaluateSnippet error: %v", err)
	}
	if want := "[\n   \"\uFFFD\",\n   \"\uFFFD\"\n]\n"; out != want {
		t.Errorf("got %q, want %q", out, want)
	}

	// The ignored names don't collide with names containing U+FFFD.
	if _, err := vm.EvaluateSnippet("t.jsonnet", `std.extVar("\uFFFD")`); err == nil {
		t.Error("ext var with an invalid UTF-8 name is reachable")
	}
}

func TestTLAVarInvalidUTF8Name(t *testing.T) {
	vm := newTestVM(t)
	vm.TLAVar("\xff", "v")

	// As in go-jsonnet, the name is decoded, so it names a missing parameter.
	_, err := vm.EvaluateSnippet("t.jsonnet", `function(x='d') x`)
	if err == nil || !strings.Contains(err.Error(), "\uFFFD") {
		t.Errorf("got %v, want an error naming parameter \uFFFD", err)
	}
	// It is ignored when the top level isn't a function.
	if _, err := vm.EvaluateSnippet("t.jsonnet", `1`); err != nil {
		t.Errorf("EvaluateSnippet error: %v", err)
	}
}

func TestNULInInputs(t *testing.T) {
	vm := newTestVM(t)
	vm.TLAVar("x", "a\x00b")
	out, err := vm.EvaluateSnippet("t.jsonnet", `function(x) std.length(x)`)
	if err != nil || out != "3\n" {
		t.Errorf("TLAVar: got %q, %v; want \"3\\n\"", out, err)
	}

	if _, err := vm.EvaluateSnippet("t.jsonnet", "1 \x00 + garbage"); err == nil {
		t.Error("snippet containing NUL: want a syntax error")
	}
}

func TestCodeErrorsAreReportedOnUse(t *testing.T) {
	vm := newTestVM(t)
	vm.ExtCode("bad", "}{")
	if _, err := vm.EvaluateSnippet("t.jsonnet", `1`); err != nil {
		t.Errorf("unused ext code: got error %v", err)
	}
	if _, err := vm.EvaluateSnippet("t.jsonnet", `std.extVar("bad")`); err == nil {
		t.Error("used ext code: want a syntax error")
	}
}

func TestNativeFunctionNilParams(t *testing.T) {
	vm := newTestVM(t)
	vm.RegisterNativeFunction("f", nil, func([]any) (any, error) { return 1, nil })
	if out, err := vm.EvaluateSnippet("t.jsonnet", `std.native("f")()`); err != nil || out != "1\n" {
		t.Errorf("got %q, %v; want 1", out, err)
	}
}
