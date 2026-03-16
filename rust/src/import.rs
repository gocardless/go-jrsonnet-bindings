use std::{
    cell::RefCell,
    collections::HashMap,
    env::current_dir,
    ffi::{c_void, CStr, CString},
    os::raw::{c_char, c_int},
    path::PathBuf,
};

use jrsonnet_evaluator::{
    bail,
    error::{ErrorKind::*, Result},
    AsPathLike, ImportResolver, ResolvePath,
};
use jrsonnet_gcmodule::Acyclic;
use jrsonnet_parser::{SourceDirectory, SourceFile, SourcePath};

use crate::VM;

/// Go import callback function pointer type.
///
/// - `ctx`: opaque user context
/// - `base`: base directory (NUL-terminated)
/// - `rel`: relative import path (NUL-terminated)
/// - `found_here`: output: resolved path (allocated by callback, freed by caller via `free()`)
/// - `buf`: output: file content buffer (allocated by callback, freed by caller via `free()`)
/// - `buflen`: output: length of `buf`
///
/// Returns 0 on error (buf/buflen contain error message), 1 on success.
pub type GoImportCallback = unsafe extern "C" fn(
    ctx: *mut c_void,
    base: *const c_char,
    rel: *const c_char,
    found_here: *mut *mut c_char,
    buf: *mut *mut c_char,
    buflen: *mut usize,
) -> c_int;

/// Import resolver that delegates to a Go callback.
#[derive(Acyclic)]
struct GoImportResolver {
    #[allow(dead_code)]
    cb: GoImportCallback,
    ctx: *mut c_void,
    out: RefCell<HashMap<SourcePath, Vec<u8>>>,
}

impl ImportResolver for GoImportResolver {
    fn resolve_from(
        &self,
        from: &SourcePath,
        path: &dyn AsPathLike,
    ) -> Result<SourcePath> {
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

        let base_cstr = CString::new(base.to_string_lossy().as_bytes())
            .unwrap_or_else(|_| CString::new(".").unwrap());

        let rel = path.as_path();
        let rel_cstr = match rel {
            ResolvePath::Str(s) => CString::new(s).unwrap(),
            ResolvePath::Path(p) => CString::new(p.to_string_lossy().as_bytes())
                .unwrap_or_else(|_| CString::new("").unwrap()),
        };

        let mut found_here: *mut c_char = std::ptr::null_mut();
        let mut buf: *mut c_char = std::ptr::null_mut();
        let mut buf_len: usize = 0;

        let success = unsafe {
            (self.cb)(
                self.ctx,
                base_cstr.as_ptr(),
                rel_cstr.as_ptr(),
                &mut found_here,
                &mut buf,
                &mut buf_len,
            )
        };

        // Copy buf contents before freeing
        let buf_data = if !buf.is_null() && buf_len > 0 {
            let slice = unsafe { std::slice::from_raw_parts(buf as *const u8, buf_len) };
            let v = slice.to_vec();
            unsafe { libc_free(buf as *mut c_void) };
            v
        } else {
            if !buf.is_null() {
                unsafe { libc_free(buf as *mut c_void) };
            }
            Vec::new()
        };

        if success == 0 {
            // Error: buf_data contains the error message
            let result = String::from_utf8(buf_data)
                .unwrap_or_else(|_| "import callback error (invalid utf-8)".to_string());
            if !found_here.is_null() {
                unsafe { libc_free(found_here as *mut c_void) };
            }
            bail!(ImportCallbackError(result));
        }

        // Success: found_here is the resolved path
        let found_here_str = if !found_here.is_null() {
            let s = unsafe { CStr::from_ptr(found_here) }
                .to_str()
                .unwrap_or("")
                .to_owned();
            unsafe { libc_free(found_here as *mut c_void) };
            s
        } else {
            String::new()
        };

        let resolved = SourcePath::new(SourceFile::new(PathBuf::from(&found_here_str)));

        let mut out = self.out.borrow_mut();
        if !out.contains_key(&resolved) {
            out.insert(resolved.clone(), buf_data);
        }

        Ok(resolved)
    }

    fn load_file_contents(&self, resolved: &SourcePath) -> Result<Vec<u8>> {
        Ok(self
            .out
            .borrow()
            .get(resolved)
            .expect("file should have been cached during resolve")
            .clone())
    }
}

extern "C" {
    // We use libc free to free memory allocated by Go's C.CString (which uses malloc)
    pub(crate) fn free(ptr: *mut c_void);
}

unsafe fn libc_free(ptr: *mut c_void) {
    unsafe { free(ptr) }
}

/// Set a custom import callback on the VM. This replaces the default file import resolver.
///
/// # Safety
///
/// `cb` must be a valid function pointer. `ctx` is passed through as-is.
#[no_mangle]
pub unsafe extern "C" fn jrsonnet_go_import_callback(
    vm: &VM,
    cb: GoImportCallback,
    ctx: *mut c_void,
) {
    vm.replace_import_resolver(GoImportResolver {
        cb,
        ctx,
        out: RefCell::new(HashMap::new()),
    });
}
