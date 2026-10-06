package conformance

import "testing"

// Evaluation results handed back to Go.
func TestOutputs(t *testing.T) {
	stringOutput := config{stringOutput: true}

	runCases(t, []testCase{
		{
			name: "string result",
			cfg:  stringOutput,
			eval: snippet(`"héllo"`),
		},
		{
			name: "string result containing NUL",
			cfg:  stringOutput,
			eval: snippet(`"a\u0000b"`),
		},
		{
			name:    "error",
			eval:    snippet(`error "boom"`),
			wantErr: true,
		},
		{
			name: "multi",
			eval: multi(`{a: 1, b: "x"}`),
		},
		{
			name: "multi hidden field",
			eval: multi(`{a: 1, b:: 2}`),
		},
		{
			name: "multi empty object",
			eval: multi(`{}`),
		},
		{
			name:    "multi non-object",
			eval:    multi(`[1]`),
			wantErr: true,
		},
		{
			name: "multi empty field name",
			eval: multi(`{"": 1}`),
		},
		{
			name: "multi value containing NUL",
			cfg:  stringOutput,
			eval: multi(`{a: "x\u0000y", b: "z"}`),
		},
		{
			name: "multi field name containing NUL",
			eval: multi(`{"a\u0000b": 1}`),
		},
		{
			name: "stream",
			eval: stream(`[1, "x", true]`),
		},
		{
			name: "stream empty array",
			eval: stream(`[]`),
		},
		{
			name:    "stream non-array",
			eval:    stream(`{}`),
			wantErr: true,
		},
	})
}
