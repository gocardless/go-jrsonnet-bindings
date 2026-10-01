package jrsonnet

/*
#include "jrsonnet_go.h"
*/
import "C"

// Names and values may contain NUL. As in go-jsonnet, each byte that isn't
// valid UTF-8 becomes U+FFFD, except that an ext var name that isn't valid
// UTF-8 is ignored, as go-jsonnet can never look it up.

// ExtVar binds an external string variable.
func (v *VM) ExtVar(key, val string) {
	_ = v.withThread(func() {
		C.jrsonnet_go_ext_var(v.vm, key, val)
	})
}

// ExtCode binds an external code variable. The code is parsed when it is
// first used, so the evaluation that uses it returns any syntax error.
func (v *VM) ExtCode(key, code string) {
	_ = v.withThread(func() {
		C.jrsonnet_go_ext_code(v.vm, key, code)
	})
}

// TLAVar binds a top-level string argument.
func (v *VM) TLAVar(key, val string) {
	_ = v.withThread(func() {
		C.jrsonnet_go_tla_var(v.vm, key, val)
	})
}

// TLACode binds a top-level code argument. The code is parsed when it is
// first used, so the evaluation that uses it returns any syntax error.
func (v *VM) TLACode(key, code string) {
	_ = v.withThread(func() {
		C.jrsonnet_go_tla_code(v.vm, key, code)
	})
}

// TLAReset clears all TLA arguments.
func (v *VM) TLAReset() {
	_ = v.withThread(func() {
		C.jrsonnet_go_tla_reset(v.vm)
	})
}
