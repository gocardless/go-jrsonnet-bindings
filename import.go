package jrsonnet

/*
#include "jrsonnet_go.h"
#include <stdlib.h>

// Forward declaration of the Go export — defined in import_bridge.c
extern int goImportCallbackBridge(uintptr_t ctx, const char *base, const char *rel,
                                   char **found_here, char **buf, uintptr_t *buflen);
*/
import "C"

import (
	"runtime/cgo"
)

// ImportFunc is the signature for custom import callbacks.
// Given a base directory and relative import path, it should return:
// - resolvedPath: the canonical path of the imported file
// - contents: the file contents
// - err: any error (will be surfaced as a Jsonnet import error)
type ImportFunc func(base, rel string) (resolvedPath string, contents []byte, err error)

//export goImportCallback
func goImportCallback(ctx C.uintptr_t, base *C.char, rel *C.char,
	foundHere **C.char, buf **C.char, buflen *C.uintptr_t) C.int {
	// ctx is the cgo.Handle value passed straight through as an integer.
	h := cgo.Handle(ctx)
	fn := h.Value().(ImportFunc)

	goBase := C.GoString(base)
	goRel := C.GoString(rel)

	resolvedPath, contents, err := fn(goBase, goRel)
	if err != nil {
		errStr := err.Error()
		*buf = C.CString(errStr)
		*buflen = C.uintptr_t(len(errStr))
		return 0
	}

	*foundHere = C.CString(resolvedPath)
	if len(contents) > 0 {
		cbuf := C.CBytes(contents)
		*buf = (*C.char)(cbuf)
	} else {
		*buf = (*C.char)(C.calloc(1, 1))
	}
	*buflen = C.uintptr_t(len(contents))
	return 1
}

// WithImporter sets a custom import callback for the VM.
func WithImporter(fn ImportFunc) Option {
	return optionFunc(func(vm *VM) error {
		h := cgo.NewHandle(fn)
		vm.importHandle = &h
		C.jrsonnet_go_import_callback(
			vm.vm,
			C.GoImportCallback(C.goImportCallbackBridge),
			C.uintptr_t(h),
		)
		return nil
	})
}
