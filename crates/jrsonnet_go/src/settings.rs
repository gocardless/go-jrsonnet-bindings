use std::os::raw::c_uint;

use jrsonnet_evaluator::{
    manifest::{JsonFormat, ManifestFormat, StringFormat},
    stack::set_stack_depth_limit,
};

use crate::{result::guard, VM};

/// go-jsonnet's default. jrsonnet's library defaults to 200, and its CLI to 512.
pub(crate) const DEFAULT_MAX_STACK: usize = 500;

/// go-jsonnet's default number of stack frames shown in an error.
pub(crate) const DEFAULT_MAX_TRACE: usize = 20;

/// JSON indented with 3 spaces, like go-jsonnet and the jrsonnet CLI.
/// `JsonFormat::default()` indents with 4.
pub(crate) fn json_format() -> Box<dyn ManifestFormat> {
    Box::new(JsonFormat::cli(3))
}

/// Set the maximum stack depth.
#[no_mangle]
pub extern "C" fn jrsonnet_go_max_stack(_vm: &VM, depth: c_uint) {
    guard(drop, || {
        set_stack_depth_limit(depth as usize);
    })
}

/// Set the maximum number of stack frames shown in an error (0 for all).
#[no_mangle]
pub extern "C" fn jrsonnet_go_max_trace(vm: &mut VM, lines: c_uint) {
    guard(drop, || {
        vm.max_trace = lines as usize;
    })
}

/// Set string output mode. When `flag` is 1, the result must be a string,
/// output as is. Like go-jsonnet and the jrsonnet CLI, any other result is an
/// error.
#[no_mangle]
pub extern "C" fn jrsonnet_go_string_output(vm: &mut VM, flag: c_uint) {
    guard(drop, || {
        vm.manifest_format = match flag {
            0 => json_format(),
            1 => Box::new(StringFormat),
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
