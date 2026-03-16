#![allow(unused_assignments)] // False positive from jrsonnet_gcmodule::Trace derive on #[trace(skip)] fields

use std::{
    ffi::{c_void, CStr, CString},
    os::raw::{c_char, c_int},
};

use jrsonnet_evaluator::{
    error::{Error, ErrorKind},
    function::builtin::{NativeCallback, NativeCallbackHandler},
    val::{ArrValue, NumValue, StrValue},
    IStr, ObjValue, Val,
};

use crate::VM;

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
    ctx: *mut c_void,
    args_json: *const c_char,
    args_json_len: usize,
    result_json: *mut *mut c_char,
    result_json_len: *mut usize,
) -> c_int;

#[derive(jrsonnet_gcmodule::Trace)]
struct GoNativeCallbackHandler {
    #[trace(skip)]
    ctx: *mut c_void,
    #[trace(skip)]
    cb: GoNativeCallback,
}

// Safety: The Go side ensures thread safety via mutex serialization.
unsafe impl Send for GoNativeCallbackHandler {}
unsafe impl Sync for GoNativeCallbackHandler {}

/// Convert a jrsonnet Val to a serde_json::Value.
fn val_to_json(val: &Val) -> serde_json::Value {
    match val {
        Val::Bool(b) => serde_json::Value::Bool(*b),
        Val::Null => serde_json::Value::Null,
        Val::Str(s) => serde_json::Value::String(s.clone().into_flat().to_string()),
        Val::Num(n) => {
            let f: f64 = n.get();
            serde_json::Number::from_f64(f)
                .map(serde_json::Value::Number)
                .unwrap_or(serde_json::Value::Null)
        }
        Val::Arr(a) => {
            let items: Vec<serde_json::Value> = a
                .iter()
                .filter_map(|item| item.ok())
                .map(|v| val_to_json(&v))
                .collect();
            serde_json::Value::Array(items)
        }
        Val::Obj(o) => {
            let mut map = serde_json::Map::new();
            for field in o.fields() {
                if let Ok(Some(v)) = o.get(field.clone()) {
                    map.insert(field.to_string(), val_to_json(&v));
                }
            }
            serde_json::Value::Object(map)
        }
        Val::Func(_) => serde_json::Value::Null,
    }
}

/// Convert a serde_json::Value back to a jrsonnet Val.
fn json_to_val(v: &serde_json::Value) -> Val {
    match v {
        serde_json::Value::Null => Val::Null,
        serde_json::Value::Bool(b) => Val::Bool(*b),
        serde_json::Value::Number(n) => {
            if let Some(f) = n.as_f64() {
                NumValue::new(f).map(Val::Num).unwrap_or(Val::Null)
            } else {
                Val::Null
            }
        }
        serde_json::Value::String(s) => Val::Str(StrValue::Flat(IStr::from(s.as_str()))),
        serde_json::Value::Array(arr) => {
            let vals: Vec<Val> = arr.iter().map(json_to_val).collect();
            Val::Arr(ArrValue::eager(vals))
        }
        serde_json::Value::Object(obj) => {
            let mut o = ObjValue::empty();
            for (k, v) in obj {
                let val = json_to_val(v);
                o.extend_field(IStr::from(k.as_str())).value(val);
            }
            Val::Obj(o)
        }
    }
}

impl NativeCallbackHandler for GoNativeCallbackHandler {
    fn call(&self, args: &[Val]) -> std::result::Result<Val, Error> {
        // Serialize args to JSON
        let json_args: Vec<serde_json::Value> = args.iter().map(val_to_json).collect();
        let json_str = serde_json::to_string(&json_args)
            .map_err(|e| Error::new(ErrorKind::RuntimeError(format!("json serialize: {e}").into())))?;

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

        // Parse JSON result back to Val
        let json_val: serde_json::Value = serde_json::from_str(&result_str)
            .map_err(|e| Error::new(ErrorKind::RuntimeError(format!("json parse: {e}").into())))?;
        Ok(json_to_val(&json_val))
    }
}

/// Register a native function on the VM.
///
/// # Safety
///
/// - `vm` must be a valid VM pointer
/// - `name` must be NUL-terminated
/// - `cb` must be a valid function pointer
/// - `params_json` must be a NUL-terminated JSON array of param name strings, e.g. `["x","y"]`
#[no_mangle]
pub unsafe extern "C" fn jrsonnet_go_native_callback(
    vm: &VM,
    name: *const c_char,
    cb: GoNativeCallback,
    ctx: *mut c_void,
    params_json: *const c_char,
) {
    let name = unsafe { CStr::from_ptr(name) }
        .to_str()
        .expect("name is not utf-8");
    let params_str = unsafe { CStr::from_ptr(params_json) }
        .to_str()
        .expect("params is not utf-8");

    let params: Vec<String> =
        serde_json::from_str(params_str).expect("params_json must be a JSON array of strings");

    let any_resolver = vm.state.context_initializer();
    any_resolver
        .as_any()
        .downcast_ref::<jrsonnet_stdlib::ContextInitializer>()
        .expect("only stdlib context initializer supported")
        .add_native(
            name,
            #[allow(deprecated)]
            NativeCallback::new(params, GoNativeCallbackHandler { ctx, cb }),
        );
}
