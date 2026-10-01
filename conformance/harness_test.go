package conformance

import (
	"errors"
	"fmt"
	"path"
	"reflect"
	"strings"
	"testing"

	jrsonnet "github.com/gocardless/go-jrsonnet-bindings"
	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
)

// config holds the VM settings shared by both engines.
type config struct {
	stringOutput bool
	noNewline    bool
	maxStack     int // 0 keeps the engine's default
	jpaths       []string
	importer     importFunc // nil keeps the engine's file importer
}

// importFunc mirrors jrsonnet.ImportFunc. The go-jsonnet engine passes the
// directory of the importing file as base.
type importFunc func(base, rel string) (foundAt string, contents []byte, err error)

type nativeFunc func(args []any) (any, error)

// engine is the VM API the cases use.
type engine interface {
	ExtVar(key, val string)
	ExtCode(key, code string)
	TLAVar(key, val string)
	TLACode(key, code string)
	TLAReset()
	Native(name string, params []string, fn nativeFunc)
	Snippet(filename, code string) (string, error)
	File(filename string) (string, error)
	SnippetMulti(filename, code string) (map[string]string, error)
	SnippetStream(filename, code string) ([]string, error)
}

// --- go-jsonnet ---

type goEngine struct{ vm *jsonnet.VM }

func newGoEngine(c config) (engine, error) {
	vm := jsonnet.MakeVM()
	vm.StringOutput = c.stringOutput
	vm.OutputNewline = !c.noNewline
	if c.maxStack > 0 {
		vm.MaxStack = c.maxStack
	}
	switch {
	case c.importer != nil:
		vm.Importer(goImporter(c.importer))
	case len(c.jpaths) > 0:
		vm.Importer(&jsonnet.FileImporter{JPaths: c.jpaths})
	}
	return &goEngine{vm: vm}, nil
}

type goImporter importFunc

func (f goImporter) Import(importedFrom, importedPath string) (jsonnet.Contents, string, error) {
	foundAt, contents, err := f(path.Dir(importedFrom), importedPath)
	if err != nil {
		return jsonnet.Contents{}, "", err
	}
	return jsonnet.MakeContentsRaw(contents), foundAt, nil
}

func (g *goEngine) ExtVar(key, val string)   { g.vm.ExtVar(key, val) }
func (g *goEngine) ExtCode(key, code string) { g.vm.ExtCode(key, code) }
func (g *goEngine) TLAVar(key, val string)   { g.vm.TLAVar(key, val) }
func (g *goEngine) TLACode(key, code string) { g.vm.TLACode(key, code) }
func (g *goEngine) TLAReset()                { g.vm.TLAReset() }

func (g *goEngine) Native(name string, params []string, fn nativeFunc) {
	ids := make(ast.Identifiers, len(params))
	for i, p := range params {
		ids[i] = ast.Identifier(p)
	}
	g.vm.NativeFunction(&jsonnet.NativeFunction{Name: name, Params: ids, Func: fn})
}

// Snippet uses EvaluateAnonymousSnippet, which resolves imports relative to
// the working directory, like this package's EvaluateSnippet.
func (g *goEngine) Snippet(filename, code string) (string, error) {
	return g.vm.EvaluateAnonymousSnippet(filename, code)
}

func (g *goEngine) File(filename string) (string, error) { return g.vm.EvaluateFile(filename) }

func (g *goEngine) SnippetMulti(filename, code string) (map[string]string, error) {
	return g.vm.EvaluateAnonymousSnippetMulti(filename, code)
}

func (g *goEngine) SnippetStream(filename, code string) ([]string, error) {
	return g.vm.EvaluateAnonymousSnippetStream(filename, code)
}

// --- go-jrsonnet-bindings ---

type jrEngine struct{ vm *jrsonnet.VM }

func newJrEngine(c config) (engine, error) {
	opts := []jrsonnet.Option{
		jrsonnet.WithStringOutput(c.stringOutput),
		jrsonnet.WithTrailingNewline(!c.noNewline),
	}
	if c.maxStack > 0 {
		opts = append(opts, jrsonnet.WithMaxStack(uint(c.maxStack)))
	}
	if c.importer != nil {
		opts = append(opts, jrsonnet.WithImporter(jrsonnet.ImportFunc(c.importer)))
	}
	if len(c.jpaths) > 0 {
		opts = append(opts, jrsonnet.WithJPath(c.jpaths...))
	}
	vm, err := jrsonnet.New(opts...)
	if err != nil {
		return nil, err
	}
	return &jrEngine{vm: vm}, nil
}

func (j *jrEngine) ExtVar(key, val string)   { j.vm.ExtVar(key, val) }
func (j *jrEngine) ExtCode(key, code string) { j.vm.ExtCode(key, code) }
func (j *jrEngine) TLAVar(key, val string)   { j.vm.TLAVar(key, val) }
func (j *jrEngine) TLACode(key, code string) { j.vm.TLACode(key, code) }
func (j *jrEngine) TLAReset()                { j.vm.TLAReset() }

func (j *jrEngine) Native(name string, params []string, fn nativeFunc) {
	j.vm.RegisterNativeFunction(name, params, jrsonnet.NativeFunc(fn))
}

func (j *jrEngine) Snippet(filename, code string) (string, error) {
	return j.vm.EvaluateSnippet(filename, code)
}

func (j *jrEngine) File(filename string) (string, error) { return j.vm.EvaluateFile(filename) }

func (j *jrEngine) SnippetMulti(filename, code string) (map[string]string, error) {
	return j.vm.EvaluateSnippetMulti(filename, code)
}

func (j *jrEngine) SnippetStream(filename, code string) ([]string, error) {
	return j.vm.EvaluateSnippetStream(filename, code)
}

// --- cases ---

type testCase struct {
	name string
	cfg  config
	// setup runs before eval, for example to bind ext vars.
	setup func(e engine)
	eval  func(e engine) (any, error)
	// wantErr is whether go-jsonnet fails. It catches a broken case that
	// both engines agree on.
	wantErr bool
	// diff names a known difference from go-jsonnet. want is then the
	// expected result from this package, or errAny for an error.
	diff string
	want any
}

var errAny = errors.New("any error")

type outcome struct {
	value any
	err   error
}

func (o outcome) String() string {
	if o.err != nil {
		// Only the first line, as stack traces are long and differ.
		msg, _, _ := strings.Cut(o.err.Error(), "\n")
		return "error: " + msg
	}
	return fmt.Sprintf("%#v", o.value)
}

func (o outcome) sameAs(other outcome) bool {
	if (o.err != nil) != (other.err != nil) {
		return false
	}
	return o.err != nil || reflect.DeepEqual(o.value, other.value)
}

func runEngine(c testCase, newEngine func(config) (engine, error)) outcome {
	e, err := newEngine(c.cfg)
	if err != nil {
		return outcome{err: fmt.Errorf("creating VM: %w", err)}
	}
	if jr, ok := e.(*jrEngine); ok {
		defer jr.vm.Close()
	}
	if c.setup != nil {
		c.setup(e)
	}
	v, err := c.eval(e)
	if err != nil {
		return outcome{err: err}
	}
	return outcome{value: normalize(v)}
}

// normalize makes empty results comparable, as go-jsonnet returns empty
// slices and maps where this package may return nil.
func normalize(v any) any {
	switch v := v.(type) {
	case []string:
		if len(v) == 0 {
			return []string{}
		}
	case map[string]string:
		if len(v) == 0 {
			return map[string]string{}
		}
	}
	return v
}

func runCases(t *testing.T, cases []testCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gj := runEngine(c, newGoEngine)
			jr := runEngine(c, newJrEngine)
			if (gj.err != nil) != c.wantErr {
				t.Fatalf("go-jsonnet: got %s, wantErr=%v, so the case is wrong", gj, c.wantErr)
			}
			if c.diff == "" {
				if !gj.sameAs(jr) {
					t.Errorf("engines differ\ngo-jsonnet: %s\njrsonnet:   %s", gj, jr)
				}
				return
			}
			want := outcome{value: c.want}
			if c.want == errAny {
				want = outcome{err: errAny}
			}
			if gj.sameAs(want) {
				t.Errorf("go-jsonnet now gives %s; remove the known difference %q", gj, c.diff)
			}
			if !jr.sameAs(want) {
				t.Errorf("known difference %q\njrsonnet: got %s, want %s", c.diff, jr, want)
			}
		})
	}
}

func snippet(code string) func(e engine) (any, error) {
	return func(e engine) (any, error) { return e.Snippet("case.jsonnet", code) }
}

func multi(code string) func(e engine) (any, error) {
	return func(e engine) (any, error) { return e.SnippetMulti("case.jsonnet", code) }
}

func stream(code string) func(e engine) (any, error) {
	return func(e engine) (any, error) { return e.SnippetStream("case.jsonnet", code) }
}
