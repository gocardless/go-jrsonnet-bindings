use std::{
    ffi::{CStr, CString},
    os::raw::{c_char, c_int},
    panic::{catch_unwind, AssertUnwindSafe},
};

use jrsonnet_evaluator::{tla::TlaArg, IStr};

use crate::{panic_message, VM};

/// Write a message to a `*mut *mut c_char` out-parameter as an owned C string.
///
/// # Safety
///
/// `err_out` must be a valid, writable pointer.
unsafe fn write_err_out(err_out: *mut *mut c_char, msg: String) {
    let cstr = CString::new(msg).unwrap_or_else(|_| CString::new("error contains NUL").unwrap());
    unsafe { *err_out = cstr.into_raw() };
}

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
    // Catch panics (e.g. non-UTF-8 input) so they never unwind across the FFI
    // boundary into Go/C, which would abort the process. Returns 1 on a caught
    // panic; the caller (Go `ExtVar`) currently ignores the code, so on bad
    // input the variable is simply not bound.
    let result = catch_unwind(AssertUnwindSafe(|| {
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
    }));
    match result {
        Ok(()) => 0,
        Err(_) => 1,
    }
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
    // Catch panics (e.g. non-UTF-8 input) so they never unwind across the FFI
    // boundary. A caught panic is reported through the existing `err_out`
    // channel, exactly like an `add_ext_code` parse error, so Go surfaces it as
    // an ordinary error rather than crashing.
    let result = catch_unwind(AssertUnwindSafe(|| {
        let name = unsafe { CStr::from_ptr(name) };
        let code = unsafe { CStr::from_ptr(code) };

        let any_initializer = vm.state.context_initializer();
        let ci = any_initializer
            .as_any()
            .downcast_ref::<jrsonnet_stdlib::ContextInitializer>()
            .expect("only stdlib context initializer supported");

        ci.add_ext_code(
            name.to_str().expect("name is not utf-8"),
            code.to_str().expect("code is not utf-8"),
        )
        .map_err(|e| format!("{e}"))
    }));

    let msg = match result {
        Ok(Ok(())) => return 0,
        Ok(Err(msg)) => msg,                 // add_ext_code parse error
        Err(panic) => panic_message(&panic), // caught panic (e.g. non-utf-8)
    };
    unsafe { write_err_out(err_out, msg) };
    1
}

/// Bind a top-level string argument. Returns 0 on success, 1 on error.
/// On error, writes error message to `err_out` (caller must free with
/// `jrsonnet_go_free_string`).
///
/// # Safety
///
/// `name`, `value` must be NUL-terminated strings.
/// `err_out` must be a valid pointer.
#[no_mangle]
pub unsafe extern "C" fn jrsonnet_go_tla_var(
    vm: &mut VM,
    name: *const c_char,
    value: *const c_char,
    err_out: *mut *mut c_char,
) -> c_int {
    // Catch panics (e.g. non-UTF-8 input) and report them through `err_out`,
    // like `jrsonnet_go_tla_code`, instead of unwinding across the FFI boundary
    // or silently dropping the binding.
    let result = catch_unwind(AssertUnwindSafe(|| {
        let name = unsafe { CStr::from_ptr(name) };
        let value = unsafe { CStr::from_ptr(value) };

        vm.tla_args.insert(
            name.to_str().expect("name is not utf-8").into(),
            TlaArg::String(value.to_str().expect("value is not utf-8").into()),
        );
    }));
    match result {
        Ok(()) => 0,
        Err(panic) => {
            unsafe { write_err_out(err_out, panic_message(&panic)) };
            1
        }
    }
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
    // Catch panics (e.g. non-UTF-8 input) and report them through `err_out`,
    // like `jrsonnet_go_ext_code`, instead of unwinding across the FFI boundary.
    let result = catch_unwind(AssertUnwindSafe(|| {
        let name_str = unsafe { CStr::from_ptr(name) }
            .to_str()
            .expect("name is not utf-8");
        let code_str = unsafe { CStr::from_ptr(code) }
            .to_str()
            .expect("code is not utf-8");

        let name_istr: IStr = name_str.into();
        vm.tla_args
            .insert(name_istr, TlaArg::InlineCode(code_str.to_owned()));
    }));
    match result {
        Ok(()) => 0,
        Err(panic) => {
            unsafe { write_err_out(err_out, panic_message(&panic)) };
            1
        }
    }
}

/// Clear all TLA arguments.
#[no_mangle]
pub extern "C" fn jrsonnet_go_tla_reset(vm: &mut VM) {
    vm.tla_args.clear();
}
