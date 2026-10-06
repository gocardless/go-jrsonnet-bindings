package jrsonnet

/*
#include "jrsonnet_go.h"
#include <stdlib.h>

// Forward declaration of the Go export — defined in import_bridge.c
extern int goImportCallbackBridge(uintptr_t ctx, const char *base, uintptr_t base_len,
                                  const char *rel, uintptr_t rel_len,
                                  char **found_here, uintptr_t *found_here_len,
                                  char **buf, uintptr_t *buf_len);
*/
import "C"

import (
	"fmt"
	"runtime/cgo"
)

// ImportFunc is the signature for custom import callbacks.
// Given a base directory and relative import path, it should return:
// - resolvedPath: the canonical path of the imported file
// - contents: the file contents
// - err: any error (will be surfaced as a Jsonnet import error)
//
// Paths are passed byte for byte. Imports that resolve to the same
// resolvedPath all use the contents first returned for it.
//
// It runs on the VM's thread during an evaluation, so it must not call the VM,
// which deadlocks, nor runtime.Goexit. A panic is returned as an error.
type ImportFunc func(base, rel string) (resolvedPath string, contents []byte, err error)

//export goImportCallback
func goImportCallback(ctx C.uintptr_t, base *C.char, baseLen C.uintptr_t, rel *C.char, relLen C.uintptr_t,
	foundHere **C.char, foundHereLen *C.uintptr_t, buf **C.char, bufLen *C.uintptr_t) C.int {
	// ctx is the cgo.Handle value passed straight through as an integer.
	fn := cgo.Handle(ctx).Value().(ImportFunc)

	resolvedPath, contents, err := callImporter(fn, C.GoStringN(base, C.int(baseLen)), C.GoStringN(rel, C.int(relLen)))
	if err != nil {
		contents = []byte(err.Error())
	}
	*buf = (*C.char)(C.CBytes(contents))
	*bufLen = C.uintptr_t(len(contents))
	if err != nil {
		return 1
	}
	*foundHere = (*C.char)(C.CBytes([]byte(resolvedPath)))
	*foundHereLen = C.uintptr_t(len(resolvedPath))
	return 0
}

// callImporter returns a panic as an error, as a panic must not unwind
// through the Rust frames that called into Go.
func callImporter(fn ImportFunc, base, rel string) (resolvedPath string, contents []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("importer panicked: %v", r)
		}
	}()
	return fn(base, rel)
}

// makeVMWithImporter creates a VM whose imports the ImportFunc behind h
// resolves.
func makeVMWithImporter(h cgo.Handle) *C.VM {
	return C.jrsonnet_go_vm_make_with_importer(C.GoImportCallback(C.goImportCallbackBridge), C.uintptr_t(h))
}

// WithImporter sets a custom import callback, which resolves every import.
// It can't be combined with WithJPath.
func WithImporter(fn ImportFunc) Option {
	return optionFunc(func(c *config) error {
		c.importer = fn
		return nil
	})
}
