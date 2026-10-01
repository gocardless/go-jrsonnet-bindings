package conformance

import "testing"

// Output modes and VM defaults.
func TestModes(t *testing.T) {
	stringOutput := config{stringOutput: true}
	noNewline := config{noNewline: true}
	// recurse(n) needs a stack depth proportional to n.
	recurse := func(n string) func(engine) (any, error) {
		return snippet(`local f(n) = if n == 0 then 0 else 1 + f(n - 1); f(` + n + `)`)
	}

	runCases(t, []testCase{
		{
			name: "JSON indentation",
			eval: snippet(`{a: [1, {b: []}], c: {}}`),
		},
		{
			name: "trailing newline disabled",
			cfg:  noNewline,
			eval: snippet(`"x"`),
		},
		{
			name:    "string output, non-string result",
			cfg:     stringOutput,
			eval:    snippet(`1`),
			wantErr: true,
		},
		{
			name:    "multi string output, non-string field",
			cfg:     stringOutput,
			eval:    multi(`{a: "x", b: 1}`),
			wantErr: true,
		},
		{
			name: "multi trailing newline disabled",
			cfg:  noNewline,
			eval: multi(`{a: 1}`),
		},
		{
			name: "stream string output",
			cfg:  stringOutput,
			eval: stream(`["a", "b"]`),
			diff: "stream mode honours string output",
			want: []string{"a\n", "b\n"},
		},
		{
			name: "stream string output, non-string element",
			cfg:  stringOutput,
			eval: stream(`["a", 1]`),
			diff: "stream mode honours string output",
			want: errAny,
		},
		{
			name: "stream trailing newline disabled",
			cfg:  noNewline,
			eval: stream(`[1]`),
			diff: "stream mode honours the trailing newline setting",
			want: []string{"1"},
		},
		{
			name: "default max stack, shallow recursion",
			eval: recurse("100"),
		},
		{
			name: "default max stack, deeper recursion",
			eval: recurse("250"),
		},
		{
			name: "default max stack, deep recursion",
			eval: recurse("450"),
		},
		{
			name:    "default max stack exceeded",
			eval:    recurse("600"),
			wantErr: true,
		},
		{
			name:    "max stack exceeded",
			cfg:     config{maxStack: 50},
			eval:    recurse("100"),
			wantErr: true,
		},
	})
}
