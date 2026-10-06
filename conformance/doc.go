// Package conformance compares go-jrsonnet-bindings with go-jsonnet.
//
// It is a separate Go module, so the bindings module doesn't depend on
// go-jsonnet.
//
// Each case runs through both engines and compares the outputs, or whether
// both returned an error. Error messages differ in wording and aren't
// compared. A case for a known difference names it in testCase.diff, which
// the README's "Differences from go-jsonnet" lists, and gives the result
// expected from this package instead.
package conformance
