pub mod import;
pub mod native;
pub mod settings;
pub mod threading;
pub mod vars_tlas;

use std::{
    any::Any,
    cell::RefCell,
    ffi::{CStr, CString},
    os::raw::{c_char, c_int},
    panic::catch_unwind,
    path::PathBuf,
    rc::Rc,
};

use jrsonnet_evaluator::{
    apply_tla,
    tla::TlaArg,
    manifest::{JsonFormat, ManifestFormat},
    rustc_hash::FxHashMap,
    trace::{CompactFormat, PathResolver, TraceFormat},
    AsPathLike, FileImportResolver, IStr, ImportResolver, Result, State,
};
use jrsonnet_gcmodule::Acyclic;
use jrsonnet_ir::SourcePath;
use jrsonnet_stdlib::ContextInitializer;

// ---------------------------------------------------------------------------
// GoResult: the return type for all fallible FFI functions
// ---------------------------------------------------------------------------

/// Result type returned across FFI. `error` codes:
/// - 0: success, `data` is the result string (caller must free with `jrsonnet_go_free_string`)
/// - 1: jrsonnet evaluation error, `data` is the error message
/// - 2: Rust panic, `data` is the panic message
#[repr(C)]
pub struct GoResult {
    pub data: *mut c_char,
    pub error: c_int,
}

impl GoResult {
    fn ok(s: String) -> Self {
        let cstr = CString::new(s).unwrap_or_else(|_| CString::new("result contains NUL").unwrap());
        GoResult {
            data: cstr.into_raw(),
            error: 0,
        }
    }
    fn err(msg: String) -> Self {
        let cstr = CString::new(msg).unwrap_or_else(|_| CString::new("error contains NUL").unwrap());
        GoResult {
            data: cstr.into_raw(),
            error: 1,
        }
    }
    fn panic(msg: String) -> Self {
        let cstr =
            CString::new(msg).unwrap_or_else(|_| CString::new("panic message contains NUL").unwrap());
        GoResult {
            data: cstr.into_raw(),
            error: 2,
        }
    }
}

// ---------------------------------------------------------------------------
// VMImportResolver: mutable wrapper so jpath_add / import_callback can swap it
// ---------------------------------------------------------------------------

#[derive(Acyclic)]
pub(crate) struct VMImportResolver {
    pub(crate) inner: RefCell<Rc<dyn ImportResolver>>,
}

impl VMImportResolver {
    fn new(value: impl ImportResolver) -> Self {
        Self {
            inner: RefCell::new(Rc::new(value)),
        }
    }
}

impl ImportResolver for VMImportResolver {
    fn load_file_contents(&self, resolved: &SourcePath) -> Result<Vec<u8>> {
        self.inner.borrow().load_file_contents(resolved)
    }

    fn resolve_from(
        &self,
        from: &SourcePath,
        path: &dyn AsPathLike,
    ) -> Result<SourcePath> {
        self.inner.borrow().resolve_from(from, path)
    }

    fn resolve_from_default(&self, path: &dyn AsPathLike) -> Result<SourcePath> {
        self.inner.borrow().resolve_from_default(path)
    }
}

// ---------------------------------------------------------------------------
// VM struct
// ---------------------------------------------------------------------------

pub struct VM {
    pub(crate) state: State,
    pub(crate) manifest_format: Box<dyn ManifestFormat>,
    pub(crate) trace_format: Box<dyn TraceFormat>,
    pub(crate) tla_args: FxHashMap<IStr, TlaArg>,
    pub(crate) trailing_newline: bool,
}

impl VM {
    pub(crate) fn replace_import_resolver(&self, resolver: impl ImportResolver) {
        *(self.state.import_resolver() as &dyn Any)
            .downcast_ref::<VMImportResolver>()
            .expect("valid resolver ty")
            .inner
            .borrow_mut() = Rc::new(resolver);
    }

    pub(crate) fn add_jpath(&self, path: PathBuf) {
        let ir = self.state.import_resolver();
        let vmi = (ir as &dyn Any)
            .downcast_ref::<VMImportResolver>()
            .expect("valid resolver ty");
        let mut rc = vmi.inner.borrow_mut();
        // Rc::get_mut gives us &mut dyn ImportResolver when refcount == 1
        let resolver_mut = Rc::get_mut(&mut *rc)
            .expect("import resolver has multiple references");
        // Cast to &mut dyn Any to downcast to concrete FileImportResolver
        let any = resolver_mut as &mut dyn Any;
        any.downcast_mut::<FileImportResolver>()
            .expect("jpaths are not compatible with callback imports")
            .add_jpath(path);
    }
}

// ---------------------------------------------------------------------------
// VM lifecycle
// ---------------------------------------------------------------------------

/// Create a new Jsonnet VM. Caller must eventually call `jrsonnet_go_vm_destroy`.
#[no_mangle]
pub extern "C" fn jrsonnet_go_vm_make() -> *mut VM {
    let mut state = State::builder();
    state
        .import_resolver(VMImportResolver::new(FileImportResolver::default()))
        .context_initializer(ContextInitializer::new(
            PathResolver::new_cwd_fallback(),
        ));
    let state = state.build();
    Box::into_raw(Box::new(VM {
        state,
        manifest_format: Box::new(JsonFormat::default()),
        trace_format: Box::new(CompactFormat::default()),
        tla_args: FxHashMap::default(),
        trailing_newline: true,
    }))
}

/// Destroy a VM previously created by `jrsonnet_go_vm_make`.
///
/// # Safety
///
/// `vm` must be a valid pointer returned by `jrsonnet_go_vm_make`.
#[no_mangle]
pub unsafe extern "C" fn jrsonnet_go_vm_destroy(vm: *mut VM) {
    if !vm.is_null() {
        drop(unsafe { Box::from_raw(vm) });
    }
}

/// Free a string returned by any `jrsonnet_go_*` function.
///
/// # Safety
///
/// `s` must be a pointer returned by a `jrsonnet_go_*` function, or null.
#[no_mangle]
pub unsafe extern "C" fn jrsonnet_go_free_string(s: *mut c_char) {
    if !s.is_null() {
        drop(unsafe { CString::from_raw(s) });
    }
}

/// Run a GC collection cycle.
#[no_mangle]
pub extern "C" fn jrsonnet_go_gc_collect() {
    let _ = jrsonnet_gcmodule::collect_thread_cycles();
}

// ---------------------------------------------------------------------------
// Evaluate
// ---------------------------------------------------------------------------

/// Evaluate a Jsonnet code snippet.
///
/// # Safety
///
/// `filename` and `snippet` must be NUL-terminated strings.
/// `vm` must be a valid pointer.
#[no_mangle]
pub unsafe extern "C" fn jrsonnet_go_evaluate_snippet(
    vm: &VM,
    filename: *const c_char,
    snippet: *const c_char,
) -> GoResult {
    let result = catch_unwind(std::panic::AssertUnwindSafe(|| {
        let filename = unsafe { CStr::from_ptr(filename) }
            .to_str()
            .expect("filename is not utf-8");
        let snippet = unsafe { CStr::from_ptr(snippet) }
            .to_str()
            .expect("snippet is not utf-8");

        let _guard = vm.state.enter();
        vm.state
            .evaluate_snippet(filename, snippet)
            .and_then(|val| apply_tla(&vm.tla_args, val))
            .and_then(|val| val.manifest(&*vm.manifest_format))
            .map(|s| append_newline(s, vm.trailing_newline))
    }));

    match result {
        Ok(Ok(output)) => GoResult::ok(output),
        Ok(Err(e)) => {
            let mut out = String::new();
            vm.trace_format.write_trace(&mut out, &e).unwrap();
            GoResult::err(out)
        }
        Err(panic) => {
            let msg = panic_message(&panic);
            GoResult::panic(msg)
        }
    }
}

/// Evaluate a Jsonnet file.
///
/// # Safety
///
/// `filename` must be a NUL-terminated string.
/// `vm` must be a valid pointer.
#[no_mangle]
pub unsafe extern "C" fn jrsonnet_go_evaluate_file(
    vm: &VM,
    filename: *const c_char,
) -> GoResult {
    let result = catch_unwind(std::panic::AssertUnwindSafe(|| {
        let filename = unsafe { CStr::from_ptr(filename) }
            .to_str()
            .expect("filename is not utf-8");
        let path = std::path::Path::new(filename);

        let _guard = vm.state.enter();
        vm.state
            .import(path)
            .and_then(|val| apply_tla(&vm.tla_args, val))
            .and_then(|val| val.manifest(&*vm.manifest_format))
            .map(|s| append_newline(s, vm.trailing_newline))
    }));

    match result {
        Ok(Ok(output)) => GoResult::ok(output),
        Ok(Err(e)) => {
            let mut out = String::new();
            vm.trace_format.write_trace(&mut out, &e).unwrap();
            GoResult::err(out)
        }
        Err(panic) => {
            let msg = panic_message(&panic);
            GoResult::panic(msg)
        }
    }
}

fn append_newline(mut s: String, enabled: bool) -> String {
    if enabled {
        s.push('\n');
    }
    s
}

fn panic_message(panic: &Box<dyn Any + Send>) -> String {
    if let Some(s) = panic.downcast_ref::<&str>() {
        s.to_string()
    } else if let Some(s) = panic.downcast_ref::<String>() {
        s.clone()
    } else {
        "unknown panic".to_string()
    }
}
