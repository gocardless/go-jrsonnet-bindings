package jrsonnet

/*
#include "jrsonnet_go.h"
#include <stdlib.h>
*/
import "C"

import "unsafe"

// ExtVar binds an external string variable.
func (v *VM) ExtVar(key, val string) {
	cKey := C.CString(key)
	cVal := C.CString(val)
	defer C.free(unsafe.Pointer(cKey))
	defer C.free(unsafe.Pointer(cVal))

	_ = v.withThread(func() {
		C.jrsonnet_go_ext_var(v.vm, cKey, cVal)
	})
}

// ExtCode binds an external code variable.
func (v *VM) ExtCode(key, code string) error {
	cKey := C.CString(key)
	cCode := C.CString(code)
	defer C.free(unsafe.Pointer(cKey))
	defer C.free(unsafe.Pointer(cCode))

	var errMsg *C.char
	var rc C.int

	err := v.withThread(func() {
		rc = C.jrsonnet_go_ext_code(v.vm, cKey, cCode, &errMsg)
	})
	if err != nil {
		return err
	}
	if rc != 0 && errMsg != nil {
		defer C.jrsonnet_go_free_string(errMsg)
		return &EvaluationError{msg: C.GoString(errMsg)}
	}
	return nil
}

// TLAVar binds a top-level string argument.
//
// It returns an error if the binding cannot be applied (for example, if key or
// val is not valid UTF-8, which jrsonnet cannot represent). Callers migrating
// from go-jsonnet, whose TLAVar cannot fail, may ignore the returned error to
// preserve the previous silently-dropped behaviour.
func (v *VM) TLAVar(key, val string) error {
	cKey := C.CString(key)
	cVal := C.CString(val)
	defer C.free(unsafe.Pointer(cKey))
	defer C.free(unsafe.Pointer(cVal))

	var errMsg *C.char
	var rc C.int

	err := v.withThread(func() {
		rc = C.jrsonnet_go_tla_var(v.vm, cKey, cVal, &errMsg)
	})
	if err != nil {
		return err
	}
	if rc != 0 && errMsg != nil {
		defer C.jrsonnet_go_free_string(errMsg)
		return &EvaluationError{msg: C.GoString(errMsg)}
	}
	return nil
}

// TLACode binds a top-level code argument.
func (v *VM) TLACode(key, code string) error {
	cKey := C.CString(key)
	cCode := C.CString(code)
	defer C.free(unsafe.Pointer(cKey))
	defer C.free(unsafe.Pointer(cCode))

	var errMsg *C.char
	var rc C.int

	err := v.withThread(func() {
		rc = C.jrsonnet_go_tla_code(v.vm, cKey, cCode, &errMsg)
	})
	if err != nil {
		return err
	}
	if rc != 0 && errMsg != nil {
		defer C.jrsonnet_go_free_string(errMsg)
		return &EvaluationError{msg: C.GoString(errMsg)}
	}
	return nil
}

// TLAReset clears all TLA arguments.
func (v *VM) TLAReset() {
	_ = v.withThread(func() {
		C.jrsonnet_go_tla_reset(v.vm)
	})
}
