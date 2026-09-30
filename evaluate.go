package jrsonnet

/*
#include "jrsonnet_go.h"
*/
import "C"

import (
	"encoding/binary"
	"unsafe"
)

// EvaluateSnippet evaluates a string of Jsonnet code and returns the result as JSON.
func (v *VM) EvaluateSnippet(filename, snippet string) (out string, err error) {
	err = v.evaluate(false, filename, snippet, C.JRSONNET_GO_MODE_SINGLE, func(b []byte) error {
		out = string(b)
		return nil
	})
	return out, err
}

// EvaluateFile evaluates a Jsonnet file and returns the result as JSON.
func (v *VM) EvaluateFile(filename string) (out string, err error) {
	err = v.evaluate(true, filename, "", C.JRSONNET_GO_MODE_SINGLE, func(b []byte) error {
		out = string(b)
		return nil
	})
	return out, err
}

// EvaluateSnippetMulti evaluates a string of Jsonnet code in multi-file mode.
// The snippet's top-level value must be an object; each field is manifested
// independently and returned as a map from field name to that field's output.
// A non-object top-level value returns an evaluation error.
func (v *VM) EvaluateSnippetMulti(filename, snippet string) (map[string]string, error) {
	return v.evaluateMulti(false, filename, snippet)
}

// EvaluateFileMulti evaluates a Jsonnet file in multi-file mode. The file's
// top-level value must be an object; each field is manifested independently and
// returned as a map from field name to that field's output. A non-object
// top-level value returns an evaluation error.
func (v *VM) EvaluateFileMulti(filename string) (map[string]string, error) {
	return v.evaluateMulti(true, filename, "")
}

// EvaluateSnippetStream evaluates a string of Jsonnet code in stream mode. The
// snippet's top-level value must be an array; each element is manifested
// independently and returned as an ordered slice (order preserved). A
// non-array top-level value returns an evaluation error.
func (v *VM) EvaluateSnippetStream(filename, snippet string) ([]string, error) {
	return v.evaluateStream(false, filename, snippet)
}

// EvaluateFileStream evaluates a Jsonnet file in stream mode. The file's
// top-level value must be an array; each element is manifested independently
// and returned as an ordered slice (order preserved). A non-array top-level
// value returns an evaluation error.
func (v *VM) EvaluateFileStream(filename string) ([]string, error) {
	return v.evaluateStream(true, filename, "")
}

func (v *VM) evaluateMulti(file bool, filename, snippet string) (out map[string]string, err error) {
	err = v.evaluate(file, filename, snippet, C.JRSONNET_GO_MODE_MULTI, func(b []byte) error {
		entries, err := decodeEntries(b)
		if err != nil {
			return err
		}
		if len(entries)%2 != 0 {
			return errMalformedResult
		}
		out = make(map[string]string, len(entries)/2)
		for i := 0; i < len(entries); i += 2 {
			out[entries[i]] = entries[i+1]
		}
		return nil
	})
	return out, err
}

func (v *VM) evaluateStream(file bool, filename, snippet string) (out []string, err error) {
	err = v.evaluate(file, filename, snippet, C.JRSONNET_GO_MODE_STREAM, func(b []byte) error {
		out, err = decodeEntries(b)
		return err
	})
	return out, err
}

// evaluate evaluates the file filename if file is true, else snippet, in the
// given mode, and passes a successful result to decode. The bytes are only
// valid until decode returns.
func (v *VM) evaluate(file bool, filename, snippet string, mode C.int, decode func([]byte) error) error {
	var r C.struct_GoResult
	err := v.withThread(func() {
		r = C.jrsonnet_go_evaluate(v.vm, C.bool(file), filename, snippet, mode)
	})
	if err != nil {
		return err
	}
	defer C.jrsonnet_go_free_result(r)

	data := unsafe.Slice((*byte)(r.data), r.len)
	switch r.error {
	case 0:
		return decode(data)
	case 2:
		return &PanicError{msg: string(data)}
	default:
		return &EvaluationError{msg: string(data)}
	}
}

var errMalformedResult = &EvaluationError{msg: "jrsonnet: malformed multi or stream result"}

// decodeEntries splits a multi or stream result into its entries. Each entry
// is a little-endian uint64 length followed by that many bytes.
func decodeEntries(b []byte) ([]string, error) {
	var entries []string
	for len(b) > 0 {
		if len(b) < 8 {
			return nil, errMalformedResult
		}
		n := binary.LittleEndian.Uint64(b)
		b = b[8:]
		if n > uint64(len(b)) {
			return nil, errMalformedResult
		}
		entries = append(entries, string(b[:n]))
		b = b[n:]
	}
	return entries, nil
}
