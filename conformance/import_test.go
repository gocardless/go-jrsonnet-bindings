package conformance

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Import resolution, through both the file importer and a Go callback.
func TestImport(t *testing.T) {
	importer := func(fn importFunc) config { return config{importer: fn} }
	// files serves files from a map.
	files := func(m map[string]string) importFunc {
		return func(_, rel string) (string, []byte, error) {
			contents, ok := m[rel]
			if !ok {
				return "", nil, errors.New("not found: " + rel)
			}
			return "/virtual/" + rel, []byte(contents), nil
		}
	}

	lib, err := filepath.Abs("../testdata/lib")
	if err != nil {
		t.Fatal(err)
	}
	// Two library paths providing the same file.
	first, second := t.TempDir(), t.TempDir()
	for dir, contents := range map[string]string{first: `"first"`, second: `"second"`} {
		if err := os.WriteFile(filepath.Join(dir, "lib.libsonnet"), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	runCases(t, []testCase{
		{
			name: "jpath",
			cfg:  config{jpaths: []string{lib}},
			eval: snippet(`(import "helper.libsonnet").greet("x")`),
		},
		{
			name: "jpath precedence",
			cfg:  config{jpaths: []string{first, second}},
			eval: snippet(`import "lib.libsonnet"`),
		},
		{
			name: "file with relative and jpath imports",
			cfg:  config{jpaths: []string{lib}},
			eval: func(e engine) (any, error) { return e.File("../testdata/diamond_import.jsonnet") },
		},
		{
			name:    "jpath missing file",
			cfg:     config{jpaths: []string{lib}},
			eval:    snippet(`import "missing.libsonnet"`),
			wantErr: true,
		},
		{
			name: "importer",
			cfg:  importer(files(map[string]string{"a.jsonnet": `"A"`})),
			eval: snippet(`import "a.jsonnet"`),
		},
		{
			name: "importer importstr",
			cfg:  importer(files(map[string]string{"a.txt": "héllo"})),
			eval: snippet(`importstr "a.txt"`),
		},
		{
			name: "importer importbin",
			cfg:  importer(files(map[string]string{"a.bin": "\xff\x00"})),
			eval: snippet(`std.join(",", std.map(std.toString, importbin "a.bin"))`),
		},
		{
			name:    "importer error",
			cfg:     importer(files(nil)),
			eval:    snippet(`import "a.jsonnet"`),
			wantErr: true,
		},
		{
			name: "importer panics",
			cfg: importer(func(_, _ string) (string, []byte, error) {
				panic("boom")
			}),
			eval:    snippet(`import "a.jsonnet"`),
			wantErr: true,
		},
		{
			name: "importer non-UTF-8 foundAt",
			cfg: importer(func(_, rel string) (string, []byte, error) {
				return "/virtual/\xff" + rel, []byte(`"` + rel + `"`), nil
			}),
			eval: snippet(`(import "a") + (import "b")`),
		},
		{
			name: "import path containing NUL",
			cfg: importer(func(_, rel string) (string, []byte, error) {
				return "/virtual/x", []byte(`1`), nil
			}),
			eval: snippet(`import "a\u0000b"`),
		},
		{
			name: "importstr non-UTF-8 file",
			cfg:  importer(files(map[string]string{"a.txt": "\xff"})),
			eval: snippet(`importstr "a.txt"`),
			diff: "importing a file that isn't valid UTF-8 is an error",
			want: errAny,
		},
	})
}
