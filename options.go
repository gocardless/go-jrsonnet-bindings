package jrsonnet

/*
#include "jrsonnet_go.h"
#include <stdlib.h>
*/
import "C"

import "unsafe"

// Option configures a VM during creation.
type Option interface {
	apply(vm *VM) error
}

type optionFunc func(vm *VM) error

func (f optionFunc) apply(vm *VM) error { return f(vm) }

// WithMaxStack sets the maximum stack depth for the VM.
func WithMaxStack(depth uint) Option {
	return optionFunc(func(vm *VM) error {
		C.jrsonnet_go_max_stack(vm.vm, C.uint(depth))
		return nil
	})
}

// WithMaxTrace sets the maximum number of stack trace lines displayed on error.
// 0 means show all.
func WithMaxTrace(lines uint) Option {
	return optionFunc(func(vm *VM) error {
		C.jrsonnet_go_max_trace(vm.vm, C.uint(lines))
		return nil
	})
}

// WithStringOutput sets whether the output should be treated as a raw string
// (true) or JSON (false).
func WithStringOutput(enabled bool) Option {
	return optionFunc(func(vm *VM) error {
		flag := C.uint(0)
		if enabled {
			flag = 1
		}
		C.jrsonnet_go_string_output(vm.vm, flag)
		return nil
	})
}

// WithJPath adds library search paths for import resolution.
func WithJPath(paths ...string) Option {
	return optionFunc(func(vm *VM) error {
		for _, p := range paths {
			cp := C.CString(p)
			C.jrsonnet_go_jpath_add(vm.vm, cp)
			C.free(unsafe.Pointer(cp))
		}
		return nil
	})
}
