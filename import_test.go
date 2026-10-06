package jrsonnet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJPathWithImporterFails(t *testing.T) {
	importer := WithImporter(func(_, _ string) (string, []byte, error) { return "", nil, nil })
	if _, err := New(WithJPath("lib"), importer); err == nil {
		t.Error("New(WithJPath, WithImporter): want an error")
	}
	if _, err := New(importer, WithJPath()); err != nil {
		t.Errorf("New(WithImporter, WithJPath()) with no paths: %v", err)
	}
}

func TestJPathContainingNULFails(t *testing.T) {
	if _, err := New(WithJPath("a\x00b")); err == nil {
		t.Error("want an error")
	}
}

func TestJPathLaterPathWins(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	for dir, contents := range map[string]string{first: `"first"`, second: `"second"`} {
		if err := os.WriteFile(filepath.Join(dir, "lib.libsonnet"), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	vm := newTestVM(t, WithJPath(first), WithJPath(second))
	out, err := vm.EvaluateSnippet("t.jsonnet", `import "lib.libsonnet"`)
	if err != nil || out != "\"second\"\n" {
		t.Errorf("got %q, %v; want the later jpath to win, as in go-jsonnet", out, err)
	}
}

func TestImporterPanic(t *testing.T) {
	vm := newTestVM(t, WithImporter(func(_, _ string) (string, []byte, error) { panic("boom") }))
	_, err := vm.EvaluateSnippet("t.jsonnet", `import "a"`)
	if err == nil || !strings.Contains(err.Error(), "importer panicked: boom") {
		t.Errorf("got %v, want the panic as an error", err)
	}
	if out, err := vm.EvaluateSnippet("t.jsonnet", `1`); err != nil || out != "1\n" {
		t.Errorf("after panic: got %q, %v", out, err)
	}
}

func TestImporterPathsAreBytes(t *testing.T) {
	var rels []string
	vm := newTestVM(t, WithImporter(func(_, rel string) (string, []byte, error) {
		rels = append(rels, rel)
		// Distinct non-UTF-8 paths must stay distinct files.
		return "/virtual/\xff" + rel, []byte(`"` + strings.ReplaceAll(rel, "\x00", "0") + `"`), nil
	}))
	out, err := vm.EvaluateSnippet("t.jsonnet", `(import "a") + (import "b") + (import "c\u0000")`)
	if err != nil || out != "\"abc0\"\n" {
		t.Errorf("got %q, %v; want \"abc0\"", out, err)
	}
	if want := []string{"a", "b", "c\x00"}; strings.Join(rels, ",") != strings.Join(want, ",") {
		t.Errorf("importer saw %q, want %q", rels, want)
	}
}

func TestLastImporterWins(t *testing.T) {
	importer := func(contents string) Option {
		return WithImporter(func(_, rel string) (string, []byte, error) { return rel, []byte(contents), nil })
	}
	vm := newTestVM(t, importer("1"), importer("2"))
	if out, err := vm.EvaluateSnippet("t.jsonnet", `import "a"`); err != nil || out != "2\n" {
		t.Errorf("got %q, %v; want 2", out, err)
	}
}
