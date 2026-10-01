package conformance

import (
	"errors"
	"testing"
)

// Native functions registered from Go.
func TestNative(t *testing.T) {
	native := func(fn nativeFunc) func(engine) {
		return func(e engine) { e.Native("f", []string{"x"}, fn) }
	}
	identity := native(func(args []any) (any, error) { return args[0], nil })

	runCases(t, []testCase{
		{
			name:  "number",
			setup: native(func(args []any) (any, error) { return args[0].(float64) * 2, nil }),
			eval:  snippet(`std.native("f")(21)`),
		},
		{
			name:  "string",
			setup: identity,
			eval:  snippet(`std.native("f")("héllo")`),
		},
		{
			name:  "nested value",
			setup: identity,
			eval:  snippet(`std.native("f")({a: [1, null, true], b: {c: 0.5}, h:: "hidden"}) == {a: [1, null, true], b: {c: 0.5}}`),
		},
		{
			name:    "returns an error",
			setup:   native(func([]any) (any, error) { return nil, errors.New("boom") }),
			eval:    snippet(`std.native("f")(1)`),
			wantErr: true,
		},
		{
			name:    "panics",
			setup:   native(func([]any) (any, error) { panic("boom") }),
			eval:    snippet(`std.native("f")(1)`),
			wantErr: true,
		},
		{
			name:    "array argument with an erroring element",
			setup:   identity,
			eval:    snippet(`std.length(std.native("f")([1, error "boom"]))`),
			wantErr: true,
		},
		{
			name:    "object argument with an erroring field",
			setup:   identity,
			eval:    snippet(`std.length(std.native("f")({a: 1, b: error "boom"}))`),
			wantErr: true,
		},
		{
			name:    "function argument",
			setup:   identity,
			eval:    snippet(`std.native("f")(function(x) x)`),
			wantErr: true,
		},
		{
			name:    "returns a non-JSON Go type",
			setup:   native(func([]any) (any, error) { return []string{"a"}, nil }),
			eval:    snippet(`std.native("f")(1)`),
			wantErr: true,
			diff:    "a native function may return anything encoding/json can marshal",
			want:    "[\n   \"a\"\n]\n",
		},
	})
}
