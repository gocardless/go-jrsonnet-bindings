//! Evaluation.
//!
//! The success payload of a [`GoResult`] depends on the mode:
//!
//! - single: the manifested value.
//! - multi: two entries per field of the top-level object, its name and its
//!   manifested value.
//! - stream: one entry per element of the top-level array, in order.
//!
//! An entry is a little-endian `u64` length followed by that many bytes.

use std::os::raw::c_int;

use jrsonnet_evaluator::{apply_tla, bail, Error, Result, StackTraceElement, Val};

use crate::{
    input::GoStr,
    result::{guard, GoResult},
    VM,
};

/// Manifest the value as a whole.
pub const JRSONNET_GO_MODE_SINGLE: c_int = 0;
/// Manifest each field of an object on its own.
pub const JRSONNET_GO_MODE_MULTI: c_int = 1;
/// Manifest each element of an array on its own.
pub const JRSONNET_GO_MODE_STREAM: c_int = 2;

/// Evaluates the file `filename` if `file` is true, else `snippet`, and
/// manifests the result in `mode`, a `JRSONNET_GO_MODE_*` constant. A file
/// path is used byte for byte. A snippet and its filename are decoded as
/// go-jsonnet does (see [`GoStr::as_str_lossy`]).
#[no_mangle]
pub extern "C" fn jrsonnet_go_evaluate(
    vm: &mut VM,
    file: bool,
    filename: GoStr,
    snippet: GoStr,
    mode: c_int,
) -> GoResult {
    guard(GoResult::panic, || {
        let _guard = vm.state.enter();
        let val = if file {
            vm.state.import(filename.as_path())
        } else {
            vm.state
                .evaluate_snippet(&*filename.as_str_lossy(), &*snippet.as_str_lossy())
        };
        match val
            .and_then(|val| apply_tla(&vm.tla_args, val))
            .and_then(|val| manifest(vm, val, mode))
        {
            Ok(data) => GoResult::ok(data),
            Err(e) => GoResult::err(format_error(vm, e)),
        }
    })
}

fn manifest(vm: &VM, val: Val, mode: c_int) -> Result<Vec<u8>> {
    match mode {
        JRSONNET_GO_MODE_SINGLE => Ok(manifest_one(vm, &val)?.into_bytes()),
        JRSONNET_GO_MODE_MULTI => {
            let Val::Obj(obj) = val else {
                bail!(
                    "multi mode: top-level value must be an object, got {}",
                    val.value_type()
                )
            };
            let mut out = Vec::new();
            for (field, data) in obj.iter() {
                push_entry(&mut out, field.as_bytes());
                push_entry(&mut out, manifest_one(vm, &data?)?.as_bytes());
            }
            Ok(out)
        }
        JRSONNET_GO_MODE_STREAM => {
            let Val::Arr(arr) = val else {
                bail!(
                    "stream mode: top-level value must be an array, got {}",
                    val.value_type()
                )
            };
            let mut out = Vec::new();
            for item in arr.iter() {
                push_entry(&mut out, manifest_one(vm, &item?)?.as_bytes());
            }
            Ok(out)
        }
        _ => bail!("unknown evaluation mode {mode}"),
    }
}

/// Manifest a value with the VM's format and trailing newline setting.
fn manifest_one(vm: &VM, val: &Val) -> Result<String> {
    let mut out = val.manifest(&*vm.manifest_format)?;
    if vm.trailing_newline {
        out.push('\n');
    }
    Ok(out)
}

fn push_entry(out: &mut Vec<u8>, entry: &[u8]) {
    out.extend_from_slice(&(entry.len() as u64).to_le_bytes());
    out.extend_from_slice(entry);
}

fn format_error(vm: &VM, mut e: Error) -> String {
    crop_trace(&mut e.trace_mut().0, vm.max_trace);
    let mut out = String::new();
    match vm.trace_format.write_trace(&mut out, &e) {
        Ok(()) => out,
        // Fails only if a Display impl fails.
        Err(_) => e.to_string(),
    }
}

/// Keeps at most `max` frames, or all if `max` is 0. Like go-jsonnet, it keeps
/// the first and last frames, with a "..." frame in between. jrsonnet never
/// reads `CompactFormat::max_trace`.
fn crop_trace(frames: &mut Vec<StackTraceElement>, max: usize) {
    if max == 0 || frames.len() <= max {
        return;
    }
    let above = max / 2;
    let below = max - above;
    let skipped = StackTraceElement {
        location: None,
        desc: "...".to_owned(),
    };
    frames.splice(above..frames.len() - below, [skipped]);
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn crops_traces_like_go_jsonnet() {
        let frames = |n: usize| -> Vec<StackTraceElement> {
            (0..n)
                .map(|i| StackTraceElement {
                    location: None,
                    desc: i.to_string(),
                })
                .collect()
        };
        let descs = |f: &[StackTraceElement]| f.iter().map(|e| e.desc.clone()).collect::<Vec<_>>();

        for (n, max, want) in [
            (5, 0, vec!["0", "1", "2", "3", "4"]),
            (5, 5, vec!["0", "1", "2", "3", "4"]),
            (5, 3, vec!["0", "...", "3", "4"]),
            (5, 4, vec!["0", "1", "...", "3", "4"]),
            (5, 1, vec!["...", "4"]),
        ] {
            let mut f = frames(n);
            crop_trace(&mut f, max);
            assert_eq!(descs(&f), want, "n={n} max={max}");
        }
    }
}
