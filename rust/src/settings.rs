use std::{
    ffi::CStr,
    os::raw::{c_char, c_uint},
    path::PathBuf,
};

use jrsonnet_evaluator::{
    manifest::{JsonFormat, ToStringFormat},
    stack::set_stack_depth_limit,
    trace::CompactFormat,
};

use crate::VM;

/// Set the maximum stack depth.
#[no_mangle]
pub extern "C" fn jrsonnet_go_max_stack(_vm: &VM, depth: c_uint) {
    set_stack_depth_limit(depth as usize);
}

/// Set the number of lines of stack trace to display (0 for all).
#[no_mangle]
pub extern "C" fn jrsonnet_go_max_trace(vm: &mut VM, lines: c_uint) {
    if let Some(format) = vm.trace_format.as_any_mut().downcast_mut::<CompactFormat>() {
        format.max_trace = lines as usize;
    }
}

/// Set string output mode. When enabled (flag=1), output is not JSON-encoded.
#[no_mangle]
pub extern "C" fn jrsonnet_go_string_output(vm: &mut VM, flag: c_uint) {
    vm.manifest_format = match flag {
        0 => Box::new(JsonFormat::default()),
        1 => Box::new(ToStringFormat),
        _ => return,
    };
}

/// Add a library search path (jpath).
///
/// # Safety
///
/// `path` must be a NUL-terminated string.
#[no_mangle]
pub unsafe extern "C" fn jrsonnet_go_jpath_add(vm: &VM, path: *const c_char) {
    let cstr = unsafe { CStr::from_ptr(path) };
    let path = PathBuf::from(cstr.to_str().expect("jpath is not utf-8"));
    vm.add_jpath(path);
}
