use std::{
    ffi::{CStr, CString},
    os::raw::{c_char, c_int},
};

use jrsonnet_evaluator::{function::TlaArg, IStr};

use crate::VM;

/// Bind an external string variable.
///
/// # Safety
///
/// `name`, `value` must be NUL-terminated strings.
#[no_mangle]
pub unsafe extern "C" fn jrsonnet_go_ext_var(
    vm: &VM,
    name: *const c_char,
    value: *const c_char,
) -> c_int {
    let name = unsafe { CStr::from_ptr(name) };
    let value = unsafe { CStr::from_ptr(value) };

    let any_initializer = vm.state.context_initializer();
    any_initializer
        .as_any()
        .downcast_ref::<jrsonnet_stdlib::ContextInitializer>()
        .expect("only stdlib context initializer supported")
        .add_ext_str(
            name.to_str().expect("name is not utf-8").into(),
            value.to_str().expect("value is not utf-8").into(),
        );
    0
}

/// Bind an external code variable. Returns 0 on success, 1 on parse error.
/// On error, writes error message to `err_out` (caller must free with `jrsonnet_go_free_string`).
///
/// # Safety
///
/// `name`, `code` must be NUL-terminated strings.
/// `err_out` must be a valid pointer.
#[no_mangle]
pub unsafe extern "C" fn jrsonnet_go_ext_code(
    vm: &VM,
    name: *const c_char,
    code: *const c_char,
    err_out: *mut *mut c_char,
) -> c_int {
    let name = unsafe { CStr::from_ptr(name) };
    let code = unsafe { CStr::from_ptr(code) };

    let any_initializer = vm.state.context_initializer();
    let ci = any_initializer
        .as_any()
        .downcast_ref::<jrsonnet_stdlib::ContextInitializer>()
        .expect("only stdlib context initializer supported");

    match ci.add_ext_code(
        name.to_str().expect("name is not utf-8"),
        code.to_str().expect("code is not utf-8"),
    ) {
        Ok(()) => 0,
        Err(e) => {
            let msg = format!("{e}");
            let cstr = CString::new(msg).unwrap_or_else(|_| CString::new("ext_code error").unwrap());
            unsafe { *err_out = cstr.into_raw() };
            1
        }
    }
}

/// Bind a top-level string argument.
///
/// # Safety
///
/// `name`, `value` must be NUL-terminated strings.
#[no_mangle]
pub unsafe extern "C" fn jrsonnet_go_tla_var(
    vm: &mut VM,
    name: *const c_char,
    value: *const c_char,
) -> c_int {
    let name = unsafe { CStr::from_ptr(name) };
    let value = unsafe { CStr::from_ptr(value) };

    vm.tla_args.insert(
        name.to_str().expect("name is not utf-8").into(),
        TlaArg::String(value.to_str().expect("value is not utf-8").into()),
    );
    0
}

/// Bind a top-level code argument. Returns 0 on success, 1 on error.
///
/// # Safety
///
/// `name`, `code` must be NUL-terminated strings.
/// `err_out` must be a valid pointer.
#[no_mangle]
pub unsafe extern "C" fn jrsonnet_go_tla_code(
    vm: &mut VM,
    name: *const c_char,
    code: *const c_char,
    err_out: *mut *mut c_char,
) -> c_int {
    let name_str = unsafe { CStr::from_ptr(name) }
        .to_str()
        .expect("name is not utf-8");
    let code_str = unsafe { CStr::from_ptr(code) }
        .to_str()
        .expect("code is not utf-8");

    let name_istr: IStr = name_str.into();
    vm.tla_args
        .insert(name_istr, TlaArg::InlineCode(code_str.to_owned()));
    let _ = err_out; // unused on success path
    0
}

/// Clear all TLA arguments.
#[no_mangle]
pub extern "C" fn jrsonnet_go_tla_reset(vm: &mut VM) {
    vm.tla_args.clear();
}
