#ifndef JRSONNET_GO_H
#define JRSONNET_GO_H

#include <stdarg.h>
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>

/**
 * Manifest the value as a whole.
 */
#define JRSONNET_GO_MODE_SINGLE 0

/**
 * Manifest each field of an object on its own.
 */
#define JRSONNET_GO_MODE_MULTI 1

/**
 * Manifest each element of an array on its own.
 */
#define JRSONNET_GO_MODE_STREAM 2

typedef struct VM VM;

/**
 * Go import callback.
 *
 * - `ctx` is the `cgo.Handle` of the Go function.
 * - `base` is the directory of the importing file, and `rel` the imported
 *   path.
 * - The callback sets `found_here` to the resolved path and `buf` to the
 *   contents, or `buf` to the error message.
 *
 * Paths are passed byte for byte. The callback allocates its outputs with
 * `malloc`, and Rust frees them. Returns 0 on success, 1 on error.
 */
typedef int (*GoImportCallback)(uintptr_t ctx,
                                const char *base,
                                uintptr_t base_len,
                                const char *rel,
                                uintptr_t rel_len,
                                char **found_here,
                                uintptr_t *found_here_len,
                                char **buf,
                                uintptr_t *buf_len);

/**
 * Bytes returned to Go. `error` is 0 on success, 1 for an evaluation error
 * and 2 for a panic, with the message in `data`. Free with
 * `jrsonnet_go_free_result`.
 */
typedef struct GoResult {
  uint8_t *data;
  uintptr_t len;
  int error;
} GoResult;

/**
 * Go native function callback type.
 *
 * - `ctx`: opaque user context
 * - `args_json`: JSON-encoded array of arguments (NUL-terminated)
 * - `args_json_len`: length of args_json (not including NUL)
 * - `result_json`: output: JSON-encoded result (allocated by callback via malloc, freed by Rust)
 * - `result_json_len`: output: length of result_json
 *
 * Returns 0 on success, 1 on error (result_json contains error message).
 */
typedef int (*GoNativeCallback)(uintptr_t ctx,
                                const char *args_json,
                                uintptr_t args_json_len,
                                char **result_json,
                                uintptr_t *result_json_len);

/**
 * Create a VM that imports from the file system, or return null if that
 * panics. Destroy it with `jrsonnet_go_vm_destroy`.
 *
 * `jpaths` holds library paths, each followed by a NUL byte, used byte for
 * byte. As in go-jsonnet, a file in a later path wins.
 */
 struct VM *jrsonnet_go_vm_make(_GoString_ jpaths);

/**
 * Create a VM whose imports `cb` resolves, or return null if that panics.
 * Destroy it with `jrsonnet_go_vm_destroy`.
 *
 * # Safety
 *
 * `cb` must be a valid function pointer. `ctx` is passed through as-is.
 */
 struct VM *jrsonnet_go_vm_make_with_importer(GoImportCallback cb, uintptr_t ctx);

/**
 * Destroy a VM previously created by `jrsonnet_go_vm_make` or
 * `jrsonnet_go_vm_make_with_importer`.
 *
 * # Safety
 *
 * `vm` must be a pointer returned by one of those, not yet destroyed.
 */
 void jrsonnet_go_vm_destroy(struct VM *vm);

/**
 * Run a GC collection cycle.
 */
 void jrsonnet_go_gc_collect(void);

/**
 * Return the number of objects the thread's cycle collector tracks, or 0 if
 * that panics. Used by tests.
 */
 uintptr_t jrsonnet_go_gc_count_tracked(void);

/**
 * Evaluates the file `filename` if `file` is true, else `snippet`, and
 * manifests the result in `mode`, a `JRSONNET_GO_MODE_*` constant. A file
 * path is used byte for byte. A snippet and its filename are decoded as
 * go-jsonnet does (see [`GoStr::as_str_lossy`]).
 *
 * An error flushes the values cached for files (see [`VM::flush_values`]).
 */

struct GoResult jrsonnet_go_evaluate(struct VM *vm,
                                     bool file,
                                     _GoString_ filename,
                                     _GoString_ snippet,
                                     int mode);

/**
 * Register a native function on the VM. A name that isn't valid UTF-8 is
 * ignored, as go-jsonnet can never look it up. `params_json` is a JSON
 * array of parameter names.
 *
 * # Safety
 *
 * `cb` must be a valid function pointer.
 */

void jrsonnet_go_native_callback(struct VM *vm,
                                 _GoString_ name,
                                 GoNativeCallback cb,
                                 uintptr_t ctx,
                                 _GoString_ params_json);

/**
 * Free a result returned by any `jrsonnet_go_*` function.
 *
 * # Safety
 *
 * `result` must come from a `jrsonnet_go_*` function and not be freed yet.
 */
 void jrsonnet_go_free_result(struct GoResult result);

/**
 * Set the maximum stack depth.
 */
 void jrsonnet_go_max_stack(const struct VM *_vm, unsigned int depth);

/**
 * Set the maximum number of stack frames shown in an error (0 for all).
 */
 void jrsonnet_go_max_trace(struct VM *vm, unsigned int lines);

/**
 * Set string output mode. When `flag` is 1, the result must be a string,
 * output as is. Like go-jsonnet and the jrsonnet CLI, any other result is an
 * error.
 */
 void jrsonnet_go_string_output(struct VM *vm, unsigned int flag);

/**
 * Set trailing-newline mode. When enabled (flag=1), a newline is appended to
 * evaluation output. Defaults to 1 (enabled).
 */
 void jrsonnet_go_trailing_newline(struct VM *vm, unsigned int flag);

/**
 * Bind an external string variable.
 */
 void jrsonnet_go_ext_var(struct VM *vm, _GoString_ name, _GoString_ value);

/**
 * Bind an external code variable.
 */
 void jrsonnet_go_ext_code(struct VM *vm, _GoString_ name, _GoString_ code);

/**
 * Bind a top-level string argument.
 */
 void jrsonnet_go_tla_var(struct VM *vm, _GoString_ name, _GoString_ value);

/**
 * Bind a top-level code argument.
 */
 void jrsonnet_go_tla_code(struct VM *vm, _GoString_ name, _GoString_ code);

/**
 * Clear all TLA arguments.
 */
 void jrsonnet_go_tla_reset(struct VM *vm);

#endif  /* JRSONNET_GO_H */
