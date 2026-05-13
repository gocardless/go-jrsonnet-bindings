// Package jrsonnet provides Go bindings for the jrsonnet Jsonnet evaluator.
package jrsonnet

/*
#cgo CFLAGS: -I${SRCDIR}/include
#cgo darwin,arm64 LDFLAGS: -L${SRCDIR}/lib/aarch64-apple-darwin -ljrsonnet_go -lm -ldl
#cgo darwin,amd64 LDFLAGS: -L${SRCDIR}/lib/aarch64-apple-darwin -ljrsonnet_go -lm -ldl
#cgo linux,amd64 LDFLAGS: -L${SRCDIR}/lib/x86_64-unknown-linux-gnu -ljrsonnet_go -lm -ldl -lpthread
#cgo linux,arm64 LDFLAGS: -L${SRCDIR}/lib/x86_64-unknown-linux-gnu -ljrsonnet_go -lm -ldl -lpthread
#include "jrsonnet_go.h"
*/
import "C"

import (
	"runtime"
	"runtime/cgo"
	"sync"
	"unsafe"
)

// VM is a Jsonnet virtual machine. It is safe for concurrent use from
// multiple goroutines — calls are serialized by an internal mutex.
type VM struct {
	vm        *C.VM
	threadCtx *C.ThreadCTX
	mu        sync.Mutex
	closed    bool

	// cgo.Handle references for callback cleanup
	importHandle  *cgo.Handle
	importCtx     unsafe.Pointer // C-allocated ctx for import callback
	nativeHandles []cgo.Handle
	nativeCtxs    []unsafe.Pointer // C-allocated ctx for each native callback
}

// New creates a new Jsonnet VM and applies the given options.
func New(opts ...Option) (*VM, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	cvm := C.jrsonnet_go_vm_make()
	ctx := C.jrsonnet_go_exit_thread()

	v := &VM{
		vm:        cvm,
		threadCtx: ctx,
	}

	// Apply options — each option needs thread context
	for _, opt := range opts {
		if err := v.applyOption(opt); err != nil {
			// Clean up on error
			v.reenter()
			C.jrsonnet_go_vm_destroy(v.vm)
			C.jrsonnet_go_gc_collect()
			C.jrsonnet_go_exit_thread()
			return nil, err
		}
	}

	return v, nil
}

// applyOption applies a single option within a locked thread.
func (v *VM) applyOption(opt Option) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	v.reenter()
	defer v.exit()
	return opt.apply(v)
}

func (v *VM) reenter() {
	C.jrsonnet_go_reenter_thread(v.threadCtx)
	v.threadCtx = nil
}

func (v *VM) exit() {
	v.threadCtx = C.jrsonnet_go_exit_thread()
}

// Close destroys the VM and frees all resources.
// It is safe to call Close multiple times.
func (v *VM) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.closed {
		return nil
	}
	v.closed = true

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	v.reenter()
	C.jrsonnet_go_vm_destroy(v.vm)
	C.jrsonnet_go_gc_collect()
	// exit thread to clean up TLS
	ctx := C.jrsonnet_go_exit_thread()
	_ = ctx

	// Release cgo handles and free C-allocated contexts
	if v.importHandle != nil {
		v.importHandle.Delete()
		C.free(v.importCtx)
	}
	for i, h := range v.nativeHandles {
		h.Delete()
		C.free(v.nativeCtxs[i])
	}

	v.vm = nil
	return nil
}
