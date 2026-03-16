package jrsonnet

/*
#include "jrsonnet_go.h"
*/
import "C"

import "runtime"

// withThread executes fn with the Rust thread context active.
// It acquires the VM mutex, locks the OS thread, restores the Rust TLS,
// calls fn, then saves the TLS again.
func (v *VM) withThread(fn func()) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.closed {
		return ErrVMClosed
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	v.reenter()
	defer v.exit()

	fn()
	return nil
}
