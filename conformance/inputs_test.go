package conformance

import "testing"

// Strings passed into the VM. go-jsonnet stores strings as runes, so each
// byte that isn't valid UTF-8 becomes one U+FFFD. NUL bytes stay.
func TestInputs(t *testing.T) {
	const valueAndLength = `local v = std.extVar("k"); v + "|" + std.length(v)`

	extVar := func(k, v string) func(engine) { return func(e engine) { e.ExtVar(k, v) } }
	tlaVar := func(k, v string) func(engine) { return func(e engine) { e.TLAVar(k, v) } }

	runCases(t, []testCase{
		{
			name:  "ext var",
			setup: extVar("k", "v"),
			eval:  snippet(`std.extVar("k")`),
		},
		{
			name:  "ext var multi-byte UTF-8",
			setup: extVar("k", "héllo ☃ 𝄞"),
			eval:  snippet(valueAndLength),
		},
		{
			name:  "ext var invalid UTF-8 value",
			setup: extVar("k", "\xff\xfe"),
			eval:  snippet(valueAndLength),
		},
		{
			name:  "ext var truncated UTF-8 sequence",
			setup: extVar("k", "a\xffb\xe2\x82c"),
			eval:  snippet(valueAndLength),
		},
		{
			name:  "ext var NUL in value",
			setup: extVar("k", "a\x00b"),
			eval:  snippet(`std.length(std.extVar("k"))`),
		},
		{
			name:    "ext var invalid UTF-8 name is unreachable",
			setup:   extVar("\xff", "v"),
			eval:    snippet(`std.extVar("\uFFFD")`),
			wantErr: true,
		},
		{
			name:  "ext var invalid UTF-8 name is ignored",
			setup: extVar("\xff", "v"),
			eval:  snippet(`1`),
		},
		{
			name:  "ext code",
			setup: func(e engine) { e.ExtCode("k", "{a: 1}") },
			eval:  snippet(`std.extVar("k").a`),
		},
		{
			name:  "ext code invalid UTF-8",
			setup: func(e engine) { e.ExtCode("k", "'\xff'") },
			eval:  snippet(`std.extVar("k")`),
		},
		{
			name:    "ext code syntax error",
			setup:   func(e engine) { e.ExtCode("k", "}{") },
			eval:    snippet(`std.extVar("k")`),
			wantErr: true,
		},
		{
			name:  "unused ext code syntax error",
			setup: func(e engine) { e.ExtCode("k", "}{") },
			eval:  snippet(`1`),
		},
		{
			name:  "tla var",
			setup: tlaVar("x", "v"),
			eval:  snippet(`function(x='d') x`),
		},
		{
			name:  "tla var invalid UTF-8 value",
			setup: tlaVar("x", "\xff"),
			eval:  snippet(`function(x='d') x`),
		},
		{
			name:  "tla var NUL in value",
			setup: tlaVar("x", "a\x00b"),
			eval:  snippet(`function(x) std.length(x)`),
		},
		{
			name:    "tla var invalid UTF-8 name",
			setup:   tlaVar("\xff", "v"),
			eval:    snippet(`function(x='d') x`),
			wantErr: true,
		},
		{
			name:  "tla var invalid UTF-8 name, top level not a function",
			setup: tlaVar("\xff", "v"),
			eval:  snippet(`1`),
		},
		{
			name:  "tla code",
			setup: func(e engine) { e.TLACode("x", "{a: 1}") },
			eval:  snippet(`function(x) x.a`),
		},
		{
			name:  "tla code invalid UTF-8",
			setup: func(e engine) { e.TLACode("x", "'\xff'") },
			eval:  snippet(`function(x) x`),
		},
		{
			name:    "tla code syntax error",
			setup:   func(e engine) { e.TLACode("x", "}{") },
			eval:    snippet(`function(x) x`),
			wantErr: true,
		},
		{
			name: "snippet invalid UTF-8",
			eval: snippet("'\xff'"),
		},
		{
			name:    "snippet NUL",
			eval:    snippet("1 \x00 + garbage"),
			wantErr: true,
		},
		{
			name: "snippet filename invalid UTF-8",
			eval: func(e engine) (any, error) { return e.Snippet("\xff.jsonnet", "1") },
		},
		{
			name: "tla reset",
			setup: func(e engine) {
				e.TLAVar("x", "v")
				e.TLAReset()
			},
			eval: snippet(`function(x='d') x`),
		},
		{
			name: "snippet std.thisFile",
			eval: snippet(`std.thisFile`),
			diff: "std.thisFile in a snippet is \"virtual:<filename>\"",
			want: "\"virtual:case.jsonnet\"\n",
		},
		{
			name:  "native function invalid UTF-8 name is ignored",
			setup: func(e engine) { e.Native("\xff", nil, func([]any) (any, error) { return 1, nil }) },
			eval:  snippet(`1`),
		},
	})
}
