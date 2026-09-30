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
 * Go import callback function pointer type.
 *
 * - `ctx`: opaque user context
 * - `base`: base directory (NUL-terminated)
 * - `rel`: relative import path (NUL-terminated)
 * - `found_here`: output: resolved path (allocated by callback, freed by caller via `free()`)
 * - `buf`: output: file content buffer (allocated by callback, freed by caller via `free()`)
 * - `buflen`: output: length of `buf`
 *
 * Returns 0 on error (buf/buflen contain error message), 1 on success.
 */
typedef int (*GoImportCallback)(uintptr_t ctx,
                                const char *base,
                                const char *rel,
                                char **found_here,
                                char **buf,
                                uintptr_t *buflen);

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
 * Create a new Jsonnet VM, or return null if that panics. Destroy it with
 * `jrsonnet_go_vm_destroy`.
 */
 struct VM *jrsonnet_go_vm_make(void);

/**
 * Destroy a VM previously created by `jrsonnet_go_vm_make`.
 *
 * # Safety
 *
 * `vm` must be a valid pointer returned by `jrsonnet_go_vm_make`.
 */
 void jrsonnet_go_vm_destroy(struct VM *vm);

/**
 * Run a GC collection cycle.
 */
 void jrsonnet_go_gc_collect(void);

/**
 * Evaluates the file `filename` if `file` is true, else `snippet`, and
 * manifests the result in `mode`, a `JRSONNET_GO_MODE_*` constant. A file
 * path is used byte for byte. A snippet and its filename are decoded as
 * go-jsonnet does (see [`GoStr::as_str_lossy`]).
 */

struct GoResult jrsonnet_go_evaluate(const struct VM *vm,
                                     bool file,
                                     _GoString_ filename,
                                     _GoString_ snippet,
                                     int mode);

extern void free(void *ptr);

/**
 * Set a custom import callback on the VM. This replaces the default file import resolver.
 *
 * # Safety
 *
 * `cb` must be a valid function pointer. `ctx` is passed through as-is.
 */
 void jrsonnet_go_import_callback(const struct VM *vm, GoImportCallback cb, uintptr_t ctx);

/**
 * Register a native function on the VM. A name that isn't valid UTF-8 is
 * ignored, as go-jsonnet can never look it up. `params_json` is a JSON
 * array of parameter names.
 *
 * # Safety
 *
 * `cb` must be a valid function pointer.
 */

void jrsonnet_go_native_callback(const struct VM *vm,
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
 * Set the number of lines of stack trace to display (0 for all).
 */
 void jrsonnet_go_max_trace(struct VM *vm, unsigned int lines);

/**
 * Set string output mode. When enabled (flag=1), output is not JSON-encoded.
 */
 void jrsonnet_go_string_output(struct VM *vm, unsigned int flag);

/**
 * Set trailing-newline mode. When enabled (flag=1), a newline is appended to
 * evaluation output. Defaults to 1 (enabled).
 */
 void jrsonnet_go_trailing_newline(struct VM *vm, unsigned int flag);

/**
 * Add a library search path (jpath). The path is used byte for byte.
 */
 void jrsonnet_go_jpath_add(const struct VM *vm, _GoString_ path);

/**
 * Bind an external string variable.
 */
 void jrsonnet_go_ext_var(const struct VM *vm, _GoString_ name, _GoString_ value);

/**
 * Bind an external code variable.
 */
 void jrsonnet_go_ext_code(const struct VM *vm, _GoString_ name, _GoString_ code);

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
