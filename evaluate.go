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

// EvaluateSnippetStream evaluates a string of Jsonnet code in stream mode. The
// snippet's top-level value must be an array; each element is manifested
// independently and returned as an ordered slice (order preserved). A
// non-array top-level value returns an evaluation error.
func (v *VM) EvaluateSnippetStream(filename, snippet string) ([]string, error) {
	cFilename := C.CString(filename)
	cSnippet := C.CString(snippet)
	defer C.free(unsafe.Pointer(cFilename))
	defer C.free(unsafe.Pointer(cSnippet))

	var result C.struct_GoResult
	err := v.withThread(func() {
		result = C.jrsonnet_go_evaluate_snippet_stream(v.vm, cFilename, cSnippet)
	})
	if err != nil {
		return nil, err
	}

	return goResultToStringSlice(result)
}

// EvaluateFileStream evaluates a Jsonnet file in stream mode. The file's
// top-level value must be an array; each element is manifested independently
// and returned as an ordered slice (order preserved). A non-array top-level
// value returns an evaluation error.
func (v *VM) EvaluateFileStream(filename string) ([]string, error) {
	cFilename := C.CString(filename)
	defer C.free(unsafe.Pointer(cFilename))

	var result C.struct_GoResult
	err := v.withThread(func() {
		result = C.jrsonnet_go_evaluate_file_stream(v.vm, cFilename)
	})
	if err != nil {
		return nil, err
	}

	return goResultToStringSlice(result)
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

// readBufferBody handles the success (code 0) transport buffer shared by the
// multi and stream evaluators: a leading little-endian uint64 total length
// followed by a NUL-separated body `token\0token\0...\0\0`. It reads the
// length, copies the whole buffer out in a single copy, frees the Rust
// allocation via the matching deallocator, and returns the body (header
// stripped) for pure-Go decoding.
func readBufferBody(r C.struct_GoResult) []byte {
	hdr := C.GoBytes(unsafe.Pointer(r.data), 8)
	total := binary.LittleEndian.Uint64(hdr)
	buf := C.GoBytes(unsafe.Pointer(r.data), C.int(total))
	C.jrsonnet_go_free_buffer(r.data, C.size_t(total))
	return buf[8:]
}

// goResultToStringMap decodes a multi-file evaluation result. Error (code 1)
// and panic (code 2) payloads are NUL-terminated C strings, freed like any
// single-value result. A success payload (code 0) is the transport buffer
// produced by the Rust side (see readBufferBody); Go only decodes it into the
// map, it never re-manifests.
func goResultToStringMap(r C.struct_GoResult) (map[string]string, error) {
	if r.data == nil {
		return nil, &EvaluationError{msg: "nil result"}
	}

	switch r.error {
	case 0:
		return decodeMulti(readBufferBody(r))
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

// goResultToStringSlice decodes a stream evaluation result, mirroring
// goResultToStringMap but yielding an ordered slice. A success payload (code
// 0) is the shared transport buffer (see readBufferBody).
func goResultToStringSlice(r C.struct_GoResult) ([]string, error) {
	if r.data == nil {
		return nil, &EvaluationError{msg: "nil result"}
	}

	switch r.error {
	case 0:
		return decodeStream(readBufferBody(r)), nil
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

// decodeStream parses the NUL-separated body of the stream transport buffer
// (the header has already been stripped by the caller) into an ordered slice.
// The body is `doc\0doc\0...\0\0`; splitting on NUL yields the docs followed by
// the empty terminator token(s), which are dropped. Order is preserved.
//
// Known, accepted limitation (shared with upstream jrsonnet's stream binding):
// the `\0\0` terminator can't distinguish an empty trailing document, nor `[]`
// from `[""]`. This is only reachable via WithStringOutput on a stream
// (non-JSON, nonsensical), so we deliberately don't frame per-token lengths.
func decodeStream(body []byte) []string {
	tokens := bytes.Split(body, []byte{0})
	out := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		if len(tok) == 0 { // empty token == terminator
			break
		}
		out = append(out, string(tok))
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
