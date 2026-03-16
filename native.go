package jrsonnet

/*
#include "jrsonnet_go.h"
#include <stdlib.h>

// Forward declaration of the Go export — defined in native_bridge.c
extern int goNativeCallbackBridge(void *ctx, const char *args_json,
                                   uintptr_t args_json_len, char **result_json,
                                   uintptr_t *result_json_len);
*/
import "C"

import (
	"encoding/json"
	"runtime/cgo"
	"unsafe"
)

// NativeFunc is the signature for native Jsonnet extension functions.
// args is a slice of deserialized JSON values (matching encoding/json conventions:
// float64 for numbers, string for strings, bool, nil, []any, map[string]any).
// The return value is serialized back to JSON.
type NativeFunc func(args []any) (any, error)

//export goNativeCallback
func goNativeCallback(ctx unsafe.Pointer, argsJSON *C.char, argsJSONLen C.uintptr_t,
	resultJSON **C.char, resultJSONLen *C.uintptr_t) C.int {
	// ctx is a C-allocated uintptr_t* holding the cgo.Handle value
	hval := *(*C.uintptr_t)(ctx)
	h := cgo.Handle(hval)
	fn := h.Value().(NativeFunc)

	// Unmarshal args from JSON
	argsBytes := C.GoBytes(unsafe.Pointer(argsJSON), C.int(argsJSONLen))
	var args []any
	if err := json.Unmarshal(argsBytes, &args); err != nil {
		errStr := "failed to unmarshal args: " + err.Error()
		*resultJSON = C.CString(errStr)
		*resultJSONLen = C.uintptr_t(len(errStr))
		return 1
	}

	// Call the Go function
	result, err := fn(args)
	if err != nil {
		errStr := err.Error()
		*resultJSON = C.CString(errStr)
		*resultJSONLen = C.uintptr_t(len(errStr))
		return 1
	}

	// Marshal result to JSON
	resultBytes, err := json.Marshal(result)
	if err != nil {
		errStr := "failed to marshal result: " + err.Error()
		*resultJSON = C.CString(errStr)
		*resultJSONLen = C.uintptr_t(len(errStr))
		return 1
	}

	*resultJSON = C.CString(string(resultBytes))
	*resultJSONLen = C.uintptr_t(len(resultBytes))
	return 0
}

// RegisterNativeFunction registers a native extension function on the VM.
// name is the function name accessible in Jsonnet as std.native("name").
// params is the list of parameter names.
// fn is the Go function that implements the extension.
func (v *VM) RegisterNativeFunction(name string, params []string, fn NativeFunc) {
	h := cgo.NewHandle(fn)
	ctx := allocHandleCtx(h)
	v.nativeHandles = append(v.nativeHandles, h)
	v.nativeCtxs = append(v.nativeCtxs, ctx)

	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	paramsJSON, _ := json.Marshal(params)
	cParams := C.CString(string(paramsJSON))
	defer C.free(unsafe.Pointer(cParams))

	_ = v.withThread(func() {
		C.jrsonnet_go_native_callback(
			v.vm,
			cName,
			C.GoNativeCallback(C.goNativeCallbackBridge),
			ctx,
			cParams,
		)
	})
}
