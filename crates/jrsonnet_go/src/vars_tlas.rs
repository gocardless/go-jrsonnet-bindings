//! External variables and top-level arguments.
//!
//! Names and values are decoded as go-jsonnet sees them (see
//! [`GoStr::as_str_lossy`]). Ext var names that aren't valid UTF-8 are
//! ignored instead, because go-jsonnet can never look them up, and decoding
//! them could collide with a name containing U+FFFD. TLA names are decoded,
//! so a bad one fails as it does in go-jsonnet.
//!
//! Code is parsed when first used, so binding it can't fail.

use jrsonnet_evaluator::tla::TlaArg;

use crate::{input::GoStr, result::guard, VM};

/// Bind an external string variable.
#[no_mangle]
pub extern "C" fn jrsonnet_go_ext_var(vm: &VM, name: GoStr, value: GoStr) {
    guard(drop, || {
        let Some(name) = name.as_str() else { return };
        vm.ctx
            .add_ext_str(name.into(), (&*value.as_str_lossy()).into());
    })
}

/// Bind an external code variable.
#[no_mangle]
pub extern "C" fn jrsonnet_go_ext_code(vm: &VM, name: GoStr, code: GoStr) {
    guard(drop, || {
        let Some(name) = name.as_str() else { return };
        let _ = vm.ctx.add_ext_code(name, code.as_str_lossy());
    })
}

/// Bind a top-level string argument.
#[no_mangle]
pub extern "C" fn jrsonnet_go_tla_var(vm: &mut VM, name: GoStr, value: GoStr) {
    guard(drop, || {
        vm.tla_args.insert(
            (&*name.as_str_lossy()).into(),
            TlaArg::String((&*value.as_str_lossy()).into()),
        );
    })
}

/// Bind a top-level code argument.
#[no_mangle]
pub extern "C" fn jrsonnet_go_tla_code(vm: &mut VM, name: GoStr, code: GoStr) {
    guard(drop, || {
        vm.tla_args.insert(
            (&*name.as_str_lossy()).into(),
            TlaArg::InlineCode(code.as_str_lossy().into_owned()),
        );
    })
}

/// Clear all TLA arguments.
#[no_mangle]
pub extern "C" fn jrsonnet_go_tla_reset(vm: &mut VM) {
    guard(drop, || {
        vm.tla_args.clear();
    })
}
