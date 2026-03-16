package jrsonnet

import "errors"

// ErrVMClosed is returned when an operation is attempted on a closed VM.
var ErrVMClosed = errors.New("jrsonnet: VM is closed")

// ErrNotImplemented is returned by stub methods that are not yet implemented.
var ErrNotImplemented = errors.New("jrsonnet: not implemented")

// EvaluationError represents a Jsonnet evaluation error.
type EvaluationError struct {
	msg string
}

func (e *EvaluationError) Error() string {
	return e.msg
}

// PanicError represents a Rust panic caught at the FFI boundary.
type PanicError struct {
	msg string
}

func (e *PanicError) Error() string {
	return "jrsonnet panic: " + e.msg
}
