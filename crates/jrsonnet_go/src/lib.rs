pub mod evaluate;
pub mod import;
pub mod input;
pub mod native;
pub mod result;
pub mod settings;
pub mod vars_tlas;

use std::{any::Any, cell::RefCell, path::PathBuf, rc::Rc};

use jrsonnet_evaluator::{
    manifest::{JsonFormat, ManifestFormat},
    rustc_hash::FxHashMap,
    tla::TlaArg,
    trace::{CompactFormat, PathResolver, TraceFormat},
    AsPathLike, FileImportResolver, IStr, ImportResolver, Result, State,
};
use jrsonnet_gcmodule::Acyclic;
use jrsonnet_ir::SourcePath;
use jrsonnet_stdlib::ContextInitializer;

use crate::result::guard;

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
    /// The context initializer in `state`. Clones share settings, so ext vars
    /// and natives added here reach `state`.
    pub(crate) ctx: ContextInitializer,
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

/// Create a new Jsonnet VM, or return null if that panics. Destroy it with
/// `jrsonnet_go_vm_destroy`.
#[no_mangle]
pub extern "C" fn jrsonnet_go_vm_make() -> *mut VM {
    guard(|_| std::ptr::null_mut(), make_vm)
}

fn make_vm() -> *mut VM {
    let ctx = ContextInitializer::new(PathResolver::new_cwd_fallback());
    let mut state = State::builder();
    state
        .import_resolver(VMImportResolver::new(FileImportResolver::default()))
        .context_initializer(ctx.clone());
    let state = state.build();
    Box::into_raw(Box::new(VM {
        state,
        ctx,
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
