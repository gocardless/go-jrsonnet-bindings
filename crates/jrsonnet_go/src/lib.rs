pub mod import;
pub mod native;
pub mod settings;
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
    AsPathLike, FileImportResolver, IStr, ImportResolver, Result, State, Val,
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

// ---------------------------------------------------------------------------
// Multi-file and stream evaluation
// ---------------------------------------------------------------------------
//
// Transport format
// ----------------
// Both multi-file and stream evaluation produce a list of manifested tokens.
// The success payload matches jrsonnet's own libjsonnet binding: a
// NUL-separated body of tokens terminated by an extra empty token. A single
// leading little-endian `u64` total length is prepended purely as transport
// framing:
//
//   [u64 total][token\0token\0...\0\0]    total = length of the whole buffer
//
// Multi feeds flattened (field name, value) pairs (`k\0v\0k\0v\0...\0\0`);
// stream feeds one manifested doc per element (`doc\0doc\0...\0\0`), preserving
// array order. Each value already carries its per-element trailing newline.
//
// The length prefix is needed because `GoResult` carries no length field and
// the body contains embedded NUL separators, so it can't be NUL-terminated /
// `strlen`'d. Go reads the leading `total` to size a single copy, splits the
// body on NUL in pure Go, and frees via `jrsonnet_go_free_buffer` (which needs
// the length for the same reason).
//
// Known, accepted limitation (shared with upstream): tokens containing a
// literal `\u0000` byte are not representable. For stream, the `\0\0`
// terminator also means an empty trailing document is indistinguishable from
// the end marker, so `[]` and `[""]` decode identically. This is only
// reachable via `WithStringOutput` on a stream (non-JSON, nonsensical) and is
// shared with upstream jrsonnet's own stream binding; we deliberately do not
// engineer around it (no per-token length framing).
//
// The `GoResult` error/panic convention is unchanged: error (code 1) and panic
// (code 2) payloads remain NUL-terminated C strings freed with
// `jrsonnet_go_free_string`; only the success payload uses this buffer shape.

/// Manifest a value in "multi" mode. The top-level value must be an object;
/// each field is manifested independently with the VM's manifest format and
/// gets the per-file trailing newline applied here (matching go-jsonnet, which
/// appends a newline to every file's content by default).
fn manifest_multi(vm: &VM, val: Val) -> Result<Vec<(IStr, String)>> {
    let Val::Obj(obj) = val else {
        return Err(jrsonnet_evaluator::error::ErrorKind::RuntimeError(
            format!(
                "multi mode: top-level value must be an object, got {}",
                val.value_type()
            )
            .into(),
        )
        .into());
    };

    let mut out = Vec::new();
    for (field, data) in obj.iter() {
        // The NUL-separated transport uses an empty key as its terminator, so
        // an empty field name can't be represented (mirrors upstream
        // jrsonnet's libjsonnet binding, whose multi format has the same
        // limitation). Fail loudly rather than silently truncate the output.
        if field.is_empty() {
            return Err(jrsonnet_evaluator::error::ErrorKind::RuntimeError(
                "multi mode: top-level field name must not be empty".into(),
            )
            .into());
        }
        let data = data?;
        let manifested = data.manifest(&*vm.manifest_format)?;
        out.push((field, append_newline(manifested, vm.trailing_newline)));
    }
    Ok(out)
}

/// Manifest a value in "stream" mode. The top-level value must be an array;
/// each element is manifested independently with the VM's manifest format,
/// preserving order, and gets the per-element trailing newline applied here
/// (matching go-jsonnet's `manifestAndSerializeYAMLStream`).
fn manifest_stream(vm: &VM, val: Val) -> Result<Vec<String>> {
    let Val::Arr(arr) = val else {
        return Err(jrsonnet_evaluator::error::ErrorKind::RuntimeError(
            format!(
                "stream mode: top-level value must be an array, got {}",
                val.value_type()
            )
            .into(),
        )
        .into());
    };

    let mut out = Vec::new();
    for item in arr.iter() {
        let manifested = item?.manifest(&*vm.manifest_format)?;
        out.push(append_newline(manifested, vm.trailing_newline));
    }
    Ok(out)
}

/// Serialize a list of tokens into the NUL-separated transport buffer with a
/// leading `u64` total-length prefix. Multi feeds flattened key/value pairs;
/// stream feeds docs. Returns a pointer to a heap buffer that must be freed
/// with `jrsonnet_go_free_buffer`.
fn tokens_to_buffer<'a>(tokens: impl Iterator<Item = &'a [u8]>) -> *mut c_char {
    let mut out: Vec<u8> = Vec::new();
    out.extend_from_slice(&[0u8; 8]); // placeholder for the total-length prefix
    for token in tokens {
        out.extend_from_slice(token);
        out.push(0);
    }
    out.push(0); // terminating empty token: `...\0\0`
    let total = out.len() as u64;
    out[..8].copy_from_slice(&total.to_le_bytes());
    // into_boxed_slice() shrinks capacity to length so the pointer/length pair
    // handed to Go is enough to reconstruct and free the allocation exactly.
    Box::into_raw(out.into_boxed_slice()) as *mut c_char
}

/// Serialize multi-file pairs into the shared transport buffer, flattening each
/// pair into consecutive `key`, `value` tokens.
fn multi_to_buffer(pairs: Vec<(IStr, String)>) -> *mut c_char {
    tokens_to_buffer(
        pairs
            .iter()
            .flat_map(|(k, v)| [k.as_bytes(), v.as_bytes()]),
    )
}

/// Serialize stream docs into the shared transport buffer, one token per doc.
fn stream_to_buffer(docs: Vec<String>) -> *mut c_char {
    tokens_to_buffer(docs.iter().map(|d| d.as_bytes()))
}

fn multi_result<F>(vm: &VM, eval: F) -> GoResult
where
    F: FnOnce() -> Result<Val>,
{
    let result = catch_unwind(std::panic::AssertUnwindSafe(|| {
        let _guard = vm.state.enter();
        eval()
            .and_then(|val| apply_tla(&vm.tla_args, val))
            .and_then(|val| manifest_multi(vm, val))
    }));

    match result {
        Ok(Ok(pairs)) => GoResult {
            data: multi_to_buffer(pairs),
            error: 0,
        },
        Ok(Err(e)) => {
            let mut out = String::new();
            vm.trace_format.write_trace(&mut out, &e).unwrap();
            GoResult::err(out)
        }
        Err(panic) => GoResult::panic(panic_message(&panic)),
    }
}

fn stream_result<F>(vm: &VM, eval: F) -> GoResult
where
    F: FnOnce() -> Result<Val>,
{
    let result = catch_unwind(std::panic::AssertUnwindSafe(|| {
        let _guard = vm.state.enter();
        eval()
            .and_then(|val| apply_tla(&vm.tla_args, val))
            .and_then(|val| manifest_stream(vm, val))
    }));

    match result {
        Ok(Ok(docs)) => GoResult {
            data: stream_to_buffer(docs),
            error: 0,
        },
        Ok(Err(e)) => {
            let mut out = String::new();
            vm.trace_format.write_trace(&mut out, &e).unwrap();
            GoResult::err(out)
        }
        Err(panic) => GoResult::panic(panic_message(&panic)),
    }
}

/// Evaluate a Jsonnet code snippet in multi-file mode. The top-level value must
/// be an object; each field is manifested independently. Success payload is the
/// length-prefixed transport buffer (see module notes), freed with
/// `jrsonnet_go_free_buffer`.
///
/// # Safety
///
/// `filename` and `snippet` must be NUL-terminated strings.
/// `vm` must be a valid pointer.
#[no_mangle]
pub unsafe extern "C" fn jrsonnet_go_evaluate_snippet_multi(
    vm: &VM,
    filename: *const c_char,
    snippet: *const c_char,
) -> GoResult {
    // Do the CStr/to_str conversions inside the caught closure (like the
    // single-value functions) so non-UTF-8 input yields a graceful panic
    // (error == 2) instead of unwinding across the FFI boundary.
    multi_result(vm, || {
        let filename = unsafe { CStr::from_ptr(filename) }
            .to_str()
            .expect("filename is not utf-8");
        let snippet = unsafe { CStr::from_ptr(snippet) }
            .to_str()
            .expect("snippet is not utf-8");
        vm.state.evaluate_snippet(filename, snippet)
    })
}

/// Evaluate a Jsonnet file in multi-file mode. The top-level value must be an
/// object; each field is manifested independently. Success payload is the
/// length-prefixed transport buffer (see module notes), freed with
/// `jrsonnet_go_free_buffer`.
///
/// # Safety
///
/// `filename` must be a NUL-terminated string.
/// `vm` must be a valid pointer.
#[no_mangle]
pub unsafe extern "C" fn jrsonnet_go_evaluate_file_multi(
    vm: &VM,
    filename: *const c_char,
) -> GoResult {
    multi_result(vm, || {
        let filename = unsafe { CStr::from_ptr(filename) }
            .to_str()
            .expect("filename is not utf-8");
        vm.state.import(std::path::Path::new(filename))
    })
}

/// Evaluate a Jsonnet code snippet in stream mode. The top-level value must be
/// an array; each element is manifested independently, preserving order.
/// Success payload is the length-prefixed transport buffer (see module notes),
/// freed with `jrsonnet_go_free_buffer`.
///
/// # Safety
///
/// `filename` and `snippet` must be NUL-terminated strings.
/// `vm` must be a valid pointer.
#[no_mangle]
pub unsafe extern "C" fn jrsonnet_go_evaluate_snippet_stream(
    vm: &VM,
    filename: *const c_char,
    snippet: *const c_char,
) -> GoResult {
    // Do the CStr/to_str conversions inside the caught closure (like the
    // single-value functions) so non-UTF-8 input yields a graceful panic
    // (error == 2) instead of unwinding across the FFI boundary.
    stream_result(vm, || {
        let filename = unsafe { CStr::from_ptr(filename) }
            .to_str()
            .expect("filename is not utf-8");
        let snippet = unsafe { CStr::from_ptr(snippet) }
            .to_str()
            .expect("snippet is not utf-8");
        vm.state.evaluate_snippet(filename, snippet)
    })
}

/// Evaluate a Jsonnet file in stream mode. The top-level value must be an
/// array; each element is manifested independently, preserving order. Success
/// payload is the length-prefixed transport buffer (see module notes), freed
/// with `jrsonnet_go_free_buffer`.
///
/// # Safety
///
/// `filename` must be a NUL-terminated string.
/// `vm` must be a valid pointer.
#[no_mangle]
pub unsafe extern "C" fn jrsonnet_go_evaluate_file_stream(
    vm: &VM,
    filename: *const c_char,
) -> GoResult {
    stream_result(vm, || {
        let filename = unsafe { CStr::from_ptr(filename) }
            .to_str()
            .expect("filename is not utf-8");
        vm.state.import(std::path::Path::new(filename))
    })
}

/// Free a multi-file / stream transport buffer returned by the `*_multi` /
/// `*_stream` evaluation functions on success (error/panic payloads are C
/// strings freed with `jrsonnet_go_free_string`).
///
/// # Safety
///
/// `data` must be a pointer returned by a multi/stream evaluation on success,
/// and `len` must be the exact total length of that buffer (its leading
/// `u64`).
#[no_mangle]
pub unsafe extern "C" fn jrsonnet_go_free_buffer(data: *mut c_char, len: usize) {
    if data.is_null() {
        return;
    }
    let slice = unsafe { std::slice::from_raw_parts_mut(data as *mut u8, len) };
    drop(unsafe { Box::from_raw(slice) });
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
