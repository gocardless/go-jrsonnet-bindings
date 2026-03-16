package jrsonnet

/*
#include "jrsonnet_go.h"
#include <stdlib.h>
*/
import "C"

import "unsafe"

// EvaluateSnippet evaluates a string of Jsonnet code and returns the result as JSON.
func (v *VM) EvaluateSnippet(filename, snippet string) (string, error) {
	cFilename := C.CString(filename)
	cSnippet := C.CString(snippet)
	defer C.free(unsafe.Pointer(cFilename))
	defer C.free(unsafe.Pointer(cSnippet))

	var result C.struct_GoResult
	err := v.withThread(func() {
		result = C.jrsonnet_go_evaluate_snippet(v.vm, cFilename, cSnippet)
	})
	if err != nil {
		return "", err
	}

	return goResultToString(result)
}

// EvaluateFile evaluates a Jsonnet file and returns the result as JSON.
func (v *VM) EvaluateFile(filename string) (string, error) {
	cFilename := C.CString(filename)
	defer C.free(unsafe.Pointer(cFilename))

	var result C.struct_GoResult
	err := v.withThread(func() {
		result = C.jrsonnet_go_evaluate_file(v.vm, cFilename)
	})
	if err != nil {
		return "", err
	}

	return goResultToString(result)
}

func goResultToString(r C.struct_GoResult) (string, error) {
	if r.data == nil {
		return "", &EvaluationError{msg: "nil result"}
	}
	defer C.jrsonnet_go_free_string(r.data)
	s := C.GoString(r.data)

	switch r.error {
	case 0:
		return s, nil
	case 1:
		return "", &EvaluationError{msg: s}
	case 2:
		return "", &PanicError{msg: s}
	default:
		return "", &EvaluationError{msg: s}
	}
}
