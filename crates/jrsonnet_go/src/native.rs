#![allow(unused_assignments)] // False positive from jrsonnet_gcmodule::Trace derive on #[trace(skip)] fields

use std::{
    ffi::{c_void, CString},
    os::raw::{c_char, c_int},
};

// `ctx` is an opaque Go `cgo.Handle` value (an integer), passed through FFI as a
// `usize`. Keeping it as an integer rather than a raw pointer means the handler
// struct is automatically `Send + Sync` (no `unsafe impl` needed) and there is
// no C-heap allocation to manage for the context.

use jrsonnet_evaluator::{
    error::{Error, ErrorKind},
    function::builtin::{NativeCallback, NativeCallbackHandler},
    manifest::JsonFormat,
    Val,
};

use crate::{input::GoStr, result::guard, VM};

/// Go native function callback type.
///
/// - `ctx`: opaque user context
/// - `args_json`: JSON-encoded array of arguments (NUL-terminated)
/// - `args_json_len`: length of args_json (not including NUL)
/// - `result_json`: output: JSON-encoded result (allocated by callback via malloc, freed by Rust)
/// - `result_json_len`: output: length of result_json
///
/// Returns 0 on success, 1 on error (result_json contains error message).
pub type GoNativeCallback = unsafe extern "C" fn(
    ctx: usize,
    args_json: *const c_char,
    args_json_len: usize,
    result_json: *mut *mut c_char,
    result_json_len: *mut usize,
) -> c_int;

#[derive(jrsonnet_gcmodule::Trace)]
struct GoNativeCallbackHandler {
    // A Go cgo.Handle value (opaque integer). Both `usize` and `extern "C" fn`
    // pointers are `Send + Sync`, so the handler is auto-`Send + Sync`; the Go
    // side still serialises all access to the VM.
    #[trace(skip)]
    ctx: usize,
    #[trace(skip)]
    cb: GoNativeCallback,
}

impl NativeCallbackHandler for GoNativeCallbackHandler {
    fn call(&self, args: &[Val]) -> std::result::Result<Val, Error> {
        // Like go-jsonnet, fail on a function or on an error anywhere in an
        // argument.
        let format = JsonFormat::minify();
        let mut json_str = String::from("[");
        for (i, arg) in args.iter().enumerate() {
            if i > 0 {
                json_str.push(',');
            }
            json_str.push_str(&arg.manifest(&format)?);
        }
        json_str.push(']');

        let json_cstr = CString::new(json_str.as_bytes())
            .map_err(|e| Error::new(ErrorKind::RuntimeError(format!("CString: {e}").into())))?;

        let mut result_ptr: *mut c_char = std::ptr::null_mut();
        let mut result_len: usize = 0;

        let status = unsafe {
            (self.cb)(
                self.ctx,
                json_cstr.as_ptr(),
                json_str.len(),
                &mut result_ptr,
                &mut result_len,
            )
        };

        // Read result
        let result_bytes = if !result_ptr.is_null() && result_len > 0 {
            let slice = unsafe { std::slice::from_raw_parts(result_ptr as *const u8, result_len) };
            let v = slice.to_vec();
            unsafe { crate::import::free(result_ptr as *mut c_void) };
            v
        } else {
            if !result_ptr.is_null() {
                unsafe { crate::import::free(result_ptr as *mut c_void) };
            }
            Vec::new()
        };

        let result_str = String::from_utf8(result_bytes)
            .map_err(|e| Error::new(ErrorKind::RuntimeError(format!("utf-8: {e}").into())))?;

        if status != 0 {
            return Err(Error::new(ErrorKind::RuntimeError(
                result_str.into(),
            )));
        }

        serde_json::from_str(&result_str)
            .map_err(|e| Error::new(ErrorKind::RuntimeError(format!("json parse: {e}").into())))
    }
}

/// Register a native function on the VM. A name that isn't valid UTF-8 is
/// ignored, as go-jsonnet can never look it up. `params_json` is a JSON
/// array of parameter names.
///
/// # Safety
///
/// `cb` must be a valid function pointer.
#[no_mangle]
pub unsafe extern "C" fn jrsonnet_go_native_callback(
    vm: &VM,
    name: GoStr,
    cb: GoNativeCallback,
    ctx: usize,
    params_json: GoStr,
) {
    guard(drop, || {
        let Some(name) = name.as_str() else { return };
        // Go marshals a nil slice as null.
        let params: Vec<String> = serde_json::from_slice::<Option<_>>(params_json.as_bytes())
            .ok()
            .flatten()
            .unwrap_or_default();
        vm.ctx.add_native(
            name,
            #[allow(deprecated)]
            NativeCallback::new(params, GoNativeCallbackHandler { ctx, cb }),
        );
    })
}
