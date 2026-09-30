package jrsonnet

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
)

func TestNULInOutputs(t *testing.T) {
	vm := newTestVM(t, WithStringOutput(true))

	out, err := vm.EvaluateSnippet("t.jsonnet", `"a\u0000b"`)
	if err != nil || out != "a\x00b\n" {
		t.Errorf("EvaluateSnippet: got %q, %v; want %q", out, err, "a\x00b\n")
	}

	multi, err := vm.EvaluateSnippetMulti("t.jsonnet", `{"k\u0000": "v\u0000", "": ""}`)
	if want := map[string]string{"k\x00": "v\x00\n", "": "\n"}; err != nil || !maps.Equal(multi, want) {
		t.Errorf("EvaluateSnippetMulti: got %q, %v; want %q", multi, err, want)
	}

	_, err = vm.EvaluateSnippet("t.jsonnet", `error "a\u0000b"`)
	var evalErr *EvaluationError
	if !errors.As(err, &evalErr) || !strings.Contains(err.Error(), "a\x00b") {
		t.Errorf("error: got %v, want an EvaluationError containing %q", err, "a\x00b")
	}
}

func TestStreamEmptyDocuments(t *testing.T) {
	vm := newTestVM(t, WithStringOutput(true), WithTrailingNewline(false))

	for _, tc := range []struct {
		snippet string
		want    []string
	}{
		{`[]`, nil},
		{`[""]`, []string{""}},
		{`["a", "", "b\u0000", ""]`, []string{"a", "", "b\x00", ""}},
	} {
		out, err := vm.EvaluateSnippetStream("t.jsonnet", tc.snippet)
		if err != nil || !slices.Equal(out, tc.want) || len(out) != len(tc.want) {
			t.Errorf("%s: got %q, %v; want %q", tc.snippet, out, err, tc.want)
		}
	}
}

func TestDecodeEntriesMalformed(t *testing.T) {
	for _, b := range [][]byte{
		{1, 0, 0},                // short length
		{2, 0, 0, 0, 0, 0, 0, 0}, // length past the end
		{2, 0, 0, 0, 0, 0, 0, 0, 'x'},
	} {
		if _, err := decodeEntries(b); err == nil {
			t.Errorf("decodeEntries(%v): want an error", b)
		}
	}
}
