use std::ptr::null_mut;

pub struct ThreadCTX {
    interner: *mut jrsonnet_interner::interop::PoolState,
    gc: *mut jrsonnet_gcmodule::interop::GcState,
}

/// Save the current thread's TLS state (interner + GC) and return a handle.
/// After this call, the current thread must NOT use jrsonnet until
/// `jrsonnet_go_reenter_thread` is called.
///
/// # Safety
///
/// Current thread GC/interner will be broken after this call.
#[no_mangle]
pub unsafe extern "C" fn jrsonnet_go_exit_thread() -> *mut ThreadCTX {
    Box::into_raw(Box::new(ThreadCTX {
        interner: jrsonnet_interner::interop::exit_thread(),
        gc: unsafe { jrsonnet_gcmodule::interop::exit_thread() },
    }))
}

/// Restore TLS state from a previously saved handle.
///
/// # Safety
///
/// `ctx` must be a valid pointer returned by `jrsonnet_go_exit_thread` and
/// must not have been used already.
#[no_mangle]
pub unsafe extern "C" fn jrsonnet_go_reenter_thread(ctx: *mut ThreadCTX) {
    let mut ctx = unsafe { Box::from_raw(ctx) };
    assert!(
        !ctx.interner.is_null() && !ctx.gc.is_null(),
        "reused context?"
    );
    unsafe { jrsonnet_interner::interop::reenter_thread(ctx.interner) }
    unsafe { jrsonnet_gcmodule::interop::reenter_thread(ctx.gc) }
    ctx.interner = null_mut();
    ctx.gc = null_mut();
}
