package jrsonnet

import (
	"fmt"
	"strings"
	"testing"
)

func TestStringOutputRequiresStrings(t *testing.T) {
	vm := newTestVM(t, WithStringOutput(true))
	if _, err := vm.EvaluateSnippet("t.jsonnet", `1`); err == nil {
		t.Error("EvaluateSnippet: want an error for a non-string result")
	}
	if _, err := vm.EvaluateSnippetMulti("t.jsonnet", `{a: "x", b: 1}`); err == nil {
		t.Error("EvaluateSnippetMulti: want an error for a non-string field")
	}
	if _, err := vm.EvaluateSnippetStream("t.jsonnet", `["x", 1]`); err == nil {
		t.Error("EvaluateSnippetStream: want an error for a non-string element")
	}
}

func TestJSONIndentation(t *testing.T) {
	vm := newTestVM(t)
	out, err := vm.EvaluateSnippet("t.jsonnet", `{a: [1]}`)
	if want := "{\n   \"a\": [\n      1\n   ]\n}\n"; err != nil || out != want {
		t.Errorf("got %q, %v; want %q", out, err, want)
	}
}

func TestDefaultMaxStack(t *testing.T) {
	const recurse = `local f(n) = if n == 0 then 0 else 1 + f(n - 1); f(%d)`
	vm := newTestVM(t)
	if _, err := vm.EvaluateSnippet("t.jsonnet", fmt.Sprintf(recurse, 450)); err != nil {
		t.Errorf("depth 450: %v", err)
	}
	if _, err := vm.EvaluateSnippet("t.jsonnet", fmt.Sprintf(recurse, 600)); err == nil {
		t.Error("depth 600: want a stack overflow")
	}

	vm = newTestVM(t, WithMaxStack(1000))
	if _, err := vm.EvaluateSnippet("t.jsonnet", fmt.Sprintf(recurse, 600)); err != nil {
		t.Errorf("WithMaxStack(1000), depth 600: %v", err)
	}
}

func TestMaxTrace(t *testing.T) {
	const deepError = `local f(n) = if n == 0 then error "boom" else 1 + f(n - 1); f(60)`
	for _, tc := range []struct {
		opts     []Option
		min, max int
	}{
		{nil, 21, 21}, // the default, 20, plus the "..." frame
		{[]Option{WithMaxTrace(4)}, 5, 5},
		{[]Option{WithMaxTrace(0)}, 61, 1000}, // all of them
	} {
		vm := newTestVM(t, tc.opts...)
		_, err := vm.EvaluateSnippet("t.jsonnet", deepError)
		if err == nil {
			t.Fatal("want an error")
		}
		// One line for the message, then one per frame.
		if frames := strings.Count(strings.TrimSpace(err.Error()), "\n"); frames < tc.min || frames > tc.max {
			t.Errorf("%d options: got %d frames, want %d to %d:\n%v", len(tc.opts), frames, tc.min, tc.max, err)
		}
	}
}
