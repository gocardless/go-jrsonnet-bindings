pub mod evaluate;
pub mod import;
pub mod input;
pub mod native;
pub mod result;
pub mod settings;
pub mod vars_tlas;

use std::path::PathBuf;

use jrsonnet_evaluator::{
    manifest::{JsonFormat, ManifestFormat},
    rustc_hash::FxHashMap,
    tla::TlaArg,
    trace::{CompactFormat, PathResolver, TraceFormat},
    FileImportResolver, IStr, ImportResolver, State,
};
use jrsonnet_stdlib::ContextInitializer;

use crate::{
    import::{GoImportCallback, GoImportResolver},
    input::{path_from_bytes, GoStr},
    result::guard,
};

pub struct VM {
    pub(crate) state: State,
    /// The context initializer in `state`. Clones share settings, so ext vars
    /// and natives added here reach `state`.
    pub(crate) ctx: ContextInitializer,
    pub(crate) manifest_format: Box<dyn ManifestFormat>,
    pub(crate) trace_format: Box<dyn TraceFormat>,
    pub(crate) tla_args: FxHashMap<IStr, TlaArg>,
    pub(crate) trailing_newline: bool,
}

/// Create a VM that imports from the file system, or return null if that
/// panics. Destroy it with `jrsonnet_go_vm_destroy`.
///
/// `jpaths` holds library paths, each followed by a NUL byte, used byte for
/// byte. As in go-jsonnet, a file in a later path wins.
#[no_mangle]
pub extern "C" fn jrsonnet_go_vm_make(jpaths: GoStr) -> *mut VM {
    guard(
        |_| std::ptr::null_mut(),
        || make_vm(FileImportResolver::new(split_jpaths(jpaths))),
    )
}

/// Create a VM whose imports `cb` resolves, or return null if that panics.
/// Destroy it with `jrsonnet_go_vm_destroy`.
///
/// # Safety
///
/// `cb` must be a valid function pointer. `ctx` is passed through as-is.
#[no_mangle]
pub unsafe extern "C" fn jrsonnet_go_vm_make_with_importer(
    cb: GoImportCallback,
    ctx: usize,
) -> *mut VM {
    guard(
        |_| std::ptr::null_mut(),
        || make_vm(GoImportResolver::new(cb, ctx)),
    )
}

fn make_vm(resolver: impl ImportResolver) -> *mut VM {
    let ctx = ContextInitializer::new(PathResolver::new_cwd_fallback());
    let mut state = State::builder();
    state
        .import_resolver(resolver)
        .context_initializer(ctx.clone());
    Box::into_raw(Box::new(VM {
        state: state.build(),
        ctx,
        manifest_format: Box::new(JsonFormat::default()),
        trace_format: Box::new(CompactFormat::default()),
        tla_args: FxHashMap::default(),
        trailing_newline: true,
    }))
}

/// Splits NUL-terminated paths and reverses them, as jrsonnet searches them
/// in order and go-jsonnet from the last one.
fn split_jpaths(jpaths: GoStr) -> Vec<PathBuf> {
    let mut out = Vec::new();
    let mut rest = jpaths.as_bytes();
    while let Some(end) = rest.iter().position(|b| *b == 0) {
        out.push(path_from_bytes(&rest[..end]).to_owned());
        rest = &rest[end + 1..];
    }
    out.reverse();
    out
}

/// Destroy a VM previously created by `jrsonnet_go_vm_make` or
/// `jrsonnet_go_vm_make_with_importer`.
///
/// # Safety
///
/// `vm` must be a pointer returned by one of those, not yet destroyed.
#[no_mangle]
pub unsafe extern "C" fn jrsonnet_go_vm_destroy(vm: *mut VM) {
    if !vm.is_null() {
        guard(drop, || drop(unsafe { Box::from_raw(vm) }))
    }
}

/// Run a GC collection cycle.
#[no_mangle]
pub extern "C" fn jrsonnet_go_gc_collect() {
    guard(drop, || {
        let _ = jrsonnet_gcmodule::collect_thread_cycles();
    })
}
