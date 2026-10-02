package jrsonnet

/*
#include "jrsonnet_go.h"
#include <stdlib.h>

// Forward declaration of the Go export — defined in native_bridge.c
extern int goNativeCallbackBridge(uintptr_t ctx, const char *args_json,
                                   uintptr_t args_json_len, char **result_json,
                                   uintptr_t *result_json_len);
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"runtime/cgo"
	"unsafe"
)

// NativeFunc is the signature for native Jsonnet extension functions.
// args is a slice of deserialized JSON values (matching encoding/json conventions:
// float64 for numbers, string for strings, bool, nil, []any, map[string]any).
// The return value is serialized back to JSON.
//
// It runs on the VM's thread during an evaluation, so it must not call the VM,
// which deadlocks, nor runtime.Goexit. A panic is returned as an error.
type NativeFunc func(args []any) (any, error)

// nativeFunc is the value behind a native function's cgo.Handle.
type nativeFunc struct {
	name string
	fn   NativeFunc
}

//export goNativeCallback
func goNativeCallback(ctx C.uintptr_t, argsJSON *C.char, argsJSONLen C.uintptr_t,
	resultJSON **C.char, resultJSONLen *C.uintptr_t) C.int {
	// ctx is the cgo.Handle value passed straight through as an integer.
	f := cgo.Handle(ctx).Value().(nativeFunc)
	args := C.GoBytes(unsafe.Pointer(argsJSON), C.int(argsJSONLen))

	out, err := f.call(args)
	if err != nil {
		out = []byte(err.Error())
	}
	*resultJSON = (*C.char)(C.CBytes(out))
	*resultJSONLen = C.uintptr_t(len(out))
	if err != nil {
		return 1
	}
	return 0
}

// call runs the function on JSON arguments and returns its JSON result. It
// returns a panic as an error, with go-jsonnet's wording, as a panic must not
// unwind through the Rust frames that called into Go.
func (f nativeFunc) call(argsJSON []byte) (result []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("native function %#v panicked: %v", f.name, r)
		}
	}()

	var args []any
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return nil, fmt.Errorf("failed to unmarshal args: %w", err)
	}
	out, err := f.fn(args)
	if err != nil {
		return nil, err
	}
	result, err = json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	return result, nil
}

// RegisterNativeFunction registers a native extension function on the VM.
// name is the function name accessible in Jsonnet as std.native("name").
// params is the list of parameter names.
// fn is the Go function that implements the extension.
func (v *VM) RegisterNativeFunction(name string, params []string, fn NativeFunc) {
	f := nativeFunc{name: name, fn: fn}

	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	paramsJSON, _ := json.Marshal(params)
	cParams := C.CString(string(paramsJSON))
	defer C.free(unsafe.Pointer(cParams))

	_ = v.withThread(func() {
		// Creating the handle on the VM's thread serialises access to
		// nativeHandles, and creates none once the VM is closed.
		h := cgo.NewHandle(f)
		v.nativeHandles = append(v.nativeHandles, h)
		C.jrsonnet_go_native_callback(
			v.vm,
			cName,
			C.GoNativeCallback(C.goNativeCallbackBridge),
			C.uintptr_t(h),
			cParams,
		)
	})
}
