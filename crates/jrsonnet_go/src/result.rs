//! Values returned to Go, and the panic guard for FFI functions.

use std::{
    any::Any,
    os::raw::c_int,
    panic::{catch_unwind, AssertUnwindSafe},
};

/// Bytes returned to Go. `error` is 0 on success, 1 for an evaluation error
/// and 2 for a panic, with the message in `data`. Free with
/// `jrsonnet_go_free_result`.
#[repr(C)]
pub struct GoResult {
    pub data: *mut u8,
    pub len: usize,
    pub error: c_int,
}

impl GoResult {
    fn new(data: Vec<u8>, error: c_int) -> Self {
        // into_boxed_slice drops spare capacity, so data and len free it.
        let data = Box::into_raw(data.into_boxed_slice());
        GoResult {
            data: data.cast(),
            len: data.len(),
            error,
        }
    }
    pub(crate) fn ok(data: Vec<u8>) -> Self {
        Self::new(data, 0)
    }
    pub(crate) fn err(msg: String) -> Self {
        Self::new(msg.into_bytes(), 1)
    }
    pub(crate) fn panic(msg: String) -> Self {
        Self::new(msg.into_bytes(), 2)
    }
}

/// Free a result returned by any `jrsonnet_go_*` function.
///
/// # Safety
///
/// `result` must come from a `jrsonnet_go_*` function and not be freed yet.
#[no_mangle]
pub unsafe extern "C" fn jrsonnet_go_free_result(result: GoResult) {
    if result.data.is_null() {
        return;
    }
    let data = std::ptr::slice_from_raw_parts_mut(result.data, result.len);
    // SAFETY: data and len come from Box::into_raw in GoResult::new.
    drop(unsafe { Box::from_raw(data) });
}

/// Runs `f`, returning `on_panic(message)` if it panics.
///
/// A panic that reaches an `extern "C"` function aborts the process, so every
/// FFI function runs in this guard. Bad input doesn't panic, so the guard only
/// catches bugs. Rust's panic hook still prints the panic to stderr.
///
/// `AssertUnwindSafe` is sound here. After a panic Go only reads the returned
/// value, and a VM left inconsistent gives wrong results, not undefined
/// behaviour.
pub(crate) fn guard<T>(on_panic: impl FnOnce(String) -> T, f: impl FnOnce() -> T) -> T {
    catch_unwind(AssertUnwindSafe(f)).unwrap_or_else(|panic| on_panic(panic_message(&*panic)))
}

fn panic_message(panic: &(dyn Any + Send)) -> String {
    if let Some(s) = panic.downcast_ref::<&str>() {
        (*s).to_string()
    } else if let Some(s) = panic.downcast_ref::<String>() {
        s.clone()
    } else {
        "unknown panic".to_string()
    }
}
