use std::os::raw::c_uint;

use jrsonnet_evaluator::{
    manifest::{JsonFormat, ToStringFormat},
    stack::set_stack_depth_limit,
    trace::CompactFormat,
};

use crate::{result::guard, VM};

/// Set the maximum stack depth.
#[no_mangle]
pub extern "C" fn jrsonnet_go_max_stack(_vm: &VM, depth: c_uint) {
    guard(drop, || {
        set_stack_depth_limit(depth as usize);
    })
}

/// Set the number of lines of stack trace to display (0 for all).
#[no_mangle]
pub extern "C" fn jrsonnet_go_max_trace(vm: &mut VM, lines: c_uint) {
    guard(drop, || {
        if let Some(format) = vm.trace_format.as_any_mut().downcast_mut::<CompactFormat>() {
            format.max_trace = lines as usize;
        }
    })
}

/// Set string output mode. When enabled (flag=1), output is not JSON-encoded.
#[no_mangle]
pub extern "C" fn jrsonnet_go_string_output(vm: &mut VM, flag: c_uint) {
    guard(drop, || {
        vm.manifest_format = match flag {
            0 => Box::new(JsonFormat::default()),
            1 => Box::new(ToStringFormat),
            _ => return,
        };
    })
}

/// Set trailing-newline mode. When enabled (flag=1), a newline is appended to
/// evaluation output. Defaults to 1 (enabled).
#[no_mangle]
pub extern "C" fn jrsonnet_go_trailing_newline(vm: &mut VM, flag: c_uint) {
    guard(drop, || {
        vm.trailing_newline = flag != 0;
    })
}
