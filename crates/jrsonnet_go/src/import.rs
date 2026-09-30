//! Import resolution through a Go callback.

use std::{
    cell::RefCell,
    collections::HashMap,
    env::current_dir,
    ffi::c_void,
    os::{
        raw::{c_char, c_int},
        unix::ffi::OsStrExt,
    },
};

use jrsonnet_evaluator::{
    bail,
    error::{ErrorKind::*, Result},
    AsPathLike, ImportResolver, ResolvePath,
};
use jrsonnet_gcmodule::Acyclic;
use jrsonnet_ir::{SourceDirectory, SourceFile, SourcePath};

use crate::input::{decode_like_go, path_from_bytes};

/// Go import callback.
///
/// - `ctx` is the `cgo.Handle` of the Go function.
/// - `base` is the directory of the importing file, and `rel` the imported
///   path.
/// - The callback sets `found_here` to the resolved path and `buf` to the
///   contents, or `buf` to the error message.
///
/// Paths are passed byte for byte. The callback allocates its outputs with
/// `malloc`, and Rust frees them. Returns 0 on success, 1 on error.
pub type GoImportCallback = unsafe extern "C" fn(
    ctx: usize,
    base: *const c_char,
    base_len: usize,
    rel: *const c_char,
    rel_len: usize,
    found_here: *mut *mut c_char,
    found_here_len: *mut usize,
    buf: *mut *mut c_char,
    buf_len: *mut usize,
) -> c_int;

/// Resolves imports through a Go callback. `ctx` is a `cgo.Handle` value,
/// passed back to the callback as is.
#[derive(Acyclic)]
pub(crate) struct GoImportResolver {
    cb: GoImportCallback,
    ctx: usize,
    /// Contents by resolved path, filled in by `resolve_from`.
    files: RefCell<HashMap<SourcePath, Vec<u8>>>,
}

impl GoImportResolver {
    pub(crate) fn new(cb: GoImportCallback, ctx: usize) -> Self {
        Self {
            cb,
            ctx,
            files: RefCell::default(),
        }
    }
}

impl ImportResolver for GoImportResolver {
    fn resolve_from(&self, from: &SourcePath, path: &dyn AsPathLike) -> Result<SourcePath> {
        let base = if let Some(p) = from.downcast_ref::<SourceFile>() {
            let mut o = p.path().to_owned();
            o.pop();
            o
        } else if let Some(d) = from.downcast_ref::<SourceDirectory>() {
            d.path().to_owned()
        } else if from.is_default() {
            current_dir().map_err(|e| ImportIo(e.to_string()))?
        } else {
            bail!(ImportIo("can't resolve this path".to_string()));
        };
        let base = base.as_os_str().as_bytes();
        let rel = match path.as_path() {
            ResolvePath::Str(s) => s.as_bytes(),
            ResolvePath::Path(p) => p.as_os_str().as_bytes(),
        };

        let mut found_here: *mut c_char = std::ptr::null_mut();
        let mut found_here_len: usize = 0;
        let mut buf: *mut c_char = std::ptr::null_mut();
        let mut buf_len: usize = 0;
        // SAFETY: Go registered `cb` with this `ctx`, and the inputs outlive the
        // call.
        let status = unsafe {
            (self.cb)(
                self.ctx,
                base.as_ptr().cast(),
                base.len(),
                rel.as_ptr().cast(),
                rel.len(),
                &mut found_here,
                &mut found_here_len,
                &mut buf,
                &mut buf_len,
            )
        };
        let found_here = take_malloced(found_here, found_here_len);
        let buf = take_malloced(buf, buf_len);

        if status != 0 {
            bail!(ImportCallbackError(decode_like_go(&buf).into_owned()));
        }

        let resolved = SourcePath::new(SourceFile::new(path_from_bytes(&found_here).to_owned()));
        // Like go-jsonnet, the first contents seen for a path win.
        self.files
            .borrow_mut()
            .entry(resolved.clone())
            .or_insert(buf);
        Ok(resolved)
    }

    fn load_file_contents(&self, resolved: &SourcePath) -> Result<Vec<u8>> {
        match self.files.borrow().get(resolved) {
            Some(contents) => Ok(contents.clone()),
            None => bail!(ImportIo(format!("{resolved} was not resolved"))),
        }
    }
}

extern "C" {
    // Frees what the Go callbacks allocate with C.CBytes.
    pub(crate) fn free(ptr: *mut c_void);
}

/// Copies and frees a buffer a Go callback allocated with `malloc`.
fn take_malloced(buf: *mut c_char, len: usize) -> Vec<u8> {
    if buf.is_null() {
        return Vec::new();
    }
    // SAFETY: Go allocated `len` bytes at `buf` with malloc and handed them
    // over.
    let data = unsafe { std::slice::from_raw_parts(buf.cast::<u8>(), len) }.to_vec();
    unsafe { free(buf.cast::<c_void>()) };
    data
}
