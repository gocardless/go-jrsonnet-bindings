package jrsonnet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, dir, name, contents string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// Like go-jsonnet, changing ext vars or natives flushes the values cached
// for files, so later evaluations see the change.
func TestReuseSeesExtVarChanges(t *testing.T) {
	dir := t.TempDir()
	file := writeFile(t, dir, "main.jsonnet", `std.extVar("k")`)
	writeFile(t, dir, "lib.libsonnet", `std.extVar("k")`)
	vm := newTestVM(t, WithJPath(dir))

	for _, v := range []string{"one", "two"} {
		vm.ExtVar("k", v)
		want := `"` + v + `"` + "\n"
		if out, err := vm.EvaluateFile(file); err != nil || out != want {
			t.Errorf("EvaluateFile after ExtVar(%q): got %q, %v", v, out, err)
		}
		if out, err := vm.EvaluateSnippet("t.jsonnet", `import "lib.libsonnet"`); err != nil || out != want {
			t.Errorf("import after ExtVar(%q): got %q, %v", v, out, err)
		}
	}

	vm.ExtCode("k", `"three"`)
	if out, err := vm.EvaluateFile(file); err != nil || out != "\"three\"\n" {
		t.Errorf("EvaluateFile after ExtCode: got %q, %v", out, err)
	}
}

// Collecting cycles must keep the values cached for imported files.
func TestReuseWithGCInterval(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lib.libsonnet", `{ a: 1, b: self.a + 1 }`)
	vm := newTestVM(t, WithJPath(dir), WithGCInterval(1))
	for i := range 3 {
		if out, err := vm.EvaluateSnippet("t.jsonnet", `(import "lib.libsonnet").b`); err != nil || out != "2\n" {
			t.Errorf("evaluation %d: got %q, %v", i, out, err)
		}
	}
}

func TestReuseSeesLateNativeFunctions(t *testing.T) {
	dir := t.TempDir()
	file := writeFile(t, dir, "main.jsonnet", `local f = std.native("f"); if f == null then "none" else f()`)
	vm := newTestVM(t)

	if out, err := vm.EvaluateFile(file); err != nil || out != "\"none\"\n" {
		t.Fatalf("before RegisterNativeFunction: got %q, %v", out, err)
	}
	vm.RegisterNativeFunction("f", nil, func([]any) (any, error) { return "f", nil })
	if out, err := vm.EvaluateFile(file); err != nil || out != "\"f\"\n" {
		t.Errorf("after RegisterNativeFunction: got %q, %v", out, err)
	}
}

// jrsonnet leaves a file that failed static analysis marked as being
// evaluated, so evaluating it again used to report infinite recursion.
func TestReuseAfterStaticError(t *testing.T) {
	file := writeFile(t, t.TempDir(), "bad.jsonnet", `local x = undefinedvar; x`)
	vm := newTestVM(t)
	for i := range 2 {
		_, err := vm.EvaluateFile(file)
		if err == nil || !strings.Contains(err.Error(), "undefinedvar") {
			t.Errorf("evaluation %d: got %v, want the undefined variable error", i, err)
		}
	}
}
