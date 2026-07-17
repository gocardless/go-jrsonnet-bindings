package jrsonnet

/*
#include "jrsonnet_go.h"
#include <stdlib.h>
*/
import "C"

import (
	"bytes"
	"encoding/binary"
	"unsafe"
)

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

// EvaluateSnippetMulti evaluates a string of Jsonnet code in multi-file mode.
// The snippet's top-level value must be an object; each field is manifested
// independently and returned as a map from field name to that field's output.
// A non-object top-level value returns an evaluation error.
func (v *VM) EvaluateSnippetMulti(filename, snippet string) (map[string]string, error) {
	cFilename := C.CString(filename)
	cSnippet := C.CString(snippet)
	defer C.free(unsafe.Pointer(cFilename))
	defer C.free(unsafe.Pointer(cSnippet))

	var result C.struct_GoResult
	err := v.withThread(func() {
		result = C.jrsonnet_go_evaluate_snippet_multi(v.vm, cFilename, cSnippet)
	})
	if err != nil {
		return nil, err
	}

	return goResultToStringMap(result)
}

// EvaluateFileMulti evaluates a Jsonnet file in multi-file mode. The file's
// top-level value must be an object; each field is manifested independently and
// returned as a map from field name to that field's output. A non-object
// top-level value returns an evaluation error.
func (v *VM) EvaluateFileMulti(filename string) (map[string]string, error) {
	cFilename := C.CString(filename)
	defer C.free(unsafe.Pointer(cFilename))

	var result C.struct_GoResult
	err := v.withThread(func() {
		result = C.jrsonnet_go_evaluate_file_multi(v.vm, cFilename)
	})
	if err != nil {
		return nil, err
	}

	return goResultToStringMap(result)
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

// goResultToStringMap decodes a multi-file evaluation result. Error (code 1)
// and panic (code 2) payloads are NUL-terminated C strings, freed like any
// single-value result. A success payload (code 0) is the multi transport
// buffer produced by the Rust side: a leading little-endian uint64 total
// length followed by a NUL-separated body `key\0value\0...\0\0`. Go only
// decodes the buffer into the map; it never re-manifests.
func goResultToStringMap(r C.struct_GoResult) (map[string]string, error) {
	if r.data == nil {
		return nil, &EvaluationError{msg: "nil result"}
	}

	switch r.error {
	case 0:
		// Success buffer: read the leading total length, copy the whole buffer
		// out, then free it with the matching multi deallocator.
		hdr := C.GoBytes(unsafe.Pointer(r.data), 8)
		total := binary.LittleEndian.Uint64(hdr)
		buf := C.GoBytes(unsafe.Pointer(r.data), C.int(total))
		C.jrsonnet_go_free_multi(r.data, C.size_t(total))
		return decodeMulti(buf[8:])
	case 1:
		defer C.jrsonnet_go_free_string(r.data)
		return nil, &EvaluationError{msg: C.GoString(r.data)}
	case 2:
		defer C.jrsonnet_go_free_string(r.data)
		return nil, &PanicError{msg: C.GoString(r.data)}
	default:
		defer C.jrsonnet_go_free_string(r.data)
		return nil, &EvaluationError{msg: C.GoString(r.data)}
	}
}

// decodeMulti parses the NUL-separated body of the multi transport buffer (the
// header has already been stripped by the caller) into a map. The body is
// `key\0value\0...\0\0`; splitting on NUL yields key/value tokens terminated by
// the empty key from the `\0\0` terminator. An empty key marks the end; an
// empty value is valid (e.g. a field manifesting to the empty string).
func decodeMulti(body []byte) (map[string]string, error) {
	tokens := bytes.Split(body, []byte{0})
	out := make(map[string]string)
	for i := 0; i < len(tokens); i += 2 {
		if len(tokens[i]) == 0 { // empty key == terminator
			break
		}
		if i+1 >= len(tokens) {
			return nil, &EvaluationError{msg: "malformed multi result buffer"}
		}
		out[string(tokens[i])] = string(tokens[i+1])
	}
	return out, nil
}
