// Minimal example program demonstrating how to build and run a consumer of
// go-jrsonnet-bindings.
//
// The point of this example is the BUILD, not the API: the static library is
// not tracked in git and must be fetched from a release artifact before the
// linker can resolve it (see the accompanying README.md, Makefile and
// fetch-lib.sh). The single evaluation below is only here to prove the binary
// links and runs.
package main

import (
	"fmt"
	"log"

	jrsonnet "github.com/gocardless/go-jrsonnet-bindings"
)

func main() {
	vm, err := jrsonnet.New()
	if err != nil {
		log.Fatal(err)
	}
	defer vm.Close()

	out, err := vm.EvaluateSnippet("example.jsonnet", `{ hello: "world" }`)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(out)
}
