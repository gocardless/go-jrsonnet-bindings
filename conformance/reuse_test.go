package conformance

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
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
	realThisFile := write("this.jsonnet", `std.thisFile`)
	invalidFile := write("invalid.jsonnet", "'\xff'")
	staticErrorFile := write("static_error.jsonnet", `local x = undefinedvar; x`)
	countFile := write("count.libsonnet", `std.native("count")()`)
	// Evaluate this.jsonnet through a symlink, so std.thisFile differs between
	// the engines on every OS.
	thisFile := filepath.Join(dir, "link.jsonnet")
	if err := os.Symlink(realThisFile, thisFile); err != nil {
		t.Fatal(err)
	}
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
			name: "file failing static analysis evaluated twice",
			eval: failedTwice(staticErrorFile, func(engine) {}),
		},
		{
			// Imported, as go-jsonnet caches the values of imported files
			// only.
			name: "imported value kept across a failed evaluation",
			setup: func(e engine) {
				n := 0
				e.Native("count", nil, func([]any) (any, error) { n++; return float64(n), nil })
			},
			eval: func(e engine) (any, error) {
				code := "import " + strconv.Quote(countFile)
				first, err1 := e.Snippet("case.jsonnet", code)
				_, _ = e.File(staticErrorFile)
				_, _ = e.Snippet("case.jsonnet", `error "boom"`)
				second, err2 := e.Snippet("case.jsonnet", code)
				return []string{first, second}, errors.Join(err1, err2)
			},
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
			name: "file evaluated twice",
			setup: func(e engine) {
				n := 0
				e.Native("count", nil, func([]any) (any, error) { n++; return float64(n), nil })
			},
			eval: evalTwice(countFile, func(engine) {}),
			diff: "the value of a file passed to EvaluateFile is cached",
			want: []string{"1\n", "1\n"},
		},
		{
			name: "std.thisFile",
			eval: func(e engine) (any, error) { return e.File(thisFile) },
			diff: "std.thisFile resolves symlinks",
			want: `"` + canonicalThisFile + `"` + "\n",
		},
	})
}
