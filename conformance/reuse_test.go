package conformance

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Evaluating files, several times on one VM.
func TestFilesAndReuse(t *testing.T) {
	dir := t.TempDir()
	write := func(name, contents string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	extVarFile := write("ext.jsonnet", `std.extVar("k")`)
	nativeFile := write("native.jsonnet", `local f = std.native("f"); if f == null then "none" else f()`)
	thisFile := write("this.jsonnet", `std.thisFile`)
	invalidFile := write("invalid.jsonnet", "'\xff'")
	canonicalThisFile, err := filepath.EvalSymlinks(thisFile)
	if err != nil {
		t.Fatal(err)
	}

	// evalTwice evaluates file, runs between, and evaluates file again.
	evalTwice := func(file string, between func(e engine)) func(e engine) (any, error) {
		return func(e engine) (any, error) {
			first, err1 := e.File(file)
			between(e)
			second, err2 := e.File(file)
			return []string{first, second}, errors.Join(err1, err2)
		}
	}
	// failedTwice is like evalTwice, but only returns whether each evaluation
	// failed.
	failedTwice := func(file string, between func(e engine)) func(e engine) (any, error) {
		return func(e engine) (any, error) {
			_, err1 := e.File(file)
			between(e)
			_, err2 := e.File(file)
			return []bool{err1 != nil, err2 != nil}, nil
		}
	}

	runCases(t, []testCase{
		{
			name:  "file",
			setup: func(e engine) { e.ExtVar("k", "v") },
			eval:  func(e engine) (any, error) { return e.File(extVarFile) },
		},
		{
			name: "failing file fixed by an ext var",
			eval: failedTwice(extVarFile, func(e engine) { e.ExtVar("k", "v") }),
		},
		{
			name:  "ext var changed between evaluations",
			setup: func(e engine) { e.ExtVar("k", "one") },
			eval:  evalTwice(extVarFile, func(e engine) { e.ExtVar("k", "two") }),
		},
		{
			name:  "ext code changed between evaluations",
			setup: func(e engine) { e.ExtCode("k", "1") },
			eval:  evalTwice(extVarFile, func(e engine) { e.ExtCode("k", "2") }),
		},
		{
			name: "native function registered between evaluations",
			eval: evalTwice(nativeFile, func(e engine) {
				e.Native("f", nil, func([]any) (any, error) { return "f", nil })
			}),
		},
		{
			name: "file that isn't valid UTF-8",
			eval: func(e engine) (any, error) { return e.File(invalidFile) },
			diff: "importing a file that isn't valid UTF-8 is an error",
			want: errAny,
		},
		{
			name: "std.thisFile",
			eval: func(e engine) (any, error) { return e.File(thisFile) },
			diff: "std.thisFile resolves symlinks",
			want: `"` + canonicalThisFile + `"` + "\n",
		},
	})
}
