#ifndef JRSONNET_GO_H
#define JRSONNET_GO_H

#include <stdarg.h>
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>

typedef struct VM VM;

/**
 * Result type returned across FFI. `error` codes:
 * - 0: success, `data` is the result string (caller must free with `jrsonnet_go_free_string`)
 * - 1: jrsonnet evaluation error, `data` is the error message
 * - 2: Rust panic, `data` is the panic message
 */
typedef struct GoResult {
  char *data;
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
typedef int (*GoImportCallback)(void *ctx,
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
typedef int (*GoNativeCallback)(void *ctx,
                                const char *args_json,
                                uintptr_t args_json_len,
                                char **result_json,
                                uintptr_t *result_json_len);

/**
 * Create a new Jsonnet VM. Caller must eventually call `jrsonnet_go_vm_destroy`.
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
 * Free a string returned by any `jrsonnet_go_*` function.
 *
 * # Safety
 *
 * `s` must be a pointer returned by a `jrsonnet_go_*` function, or null.
 */
 void jrsonnet_go_free_string(char *s);

/**
 * Run a GC collection cycle.
 */
 void jrsonnet_go_gc_collect(void);

/**
 * Evaluate a Jsonnet code snippet.
 *
 * # Safety
 *
 * `filename` and `snippet` must be NUL-terminated strings.
 * `vm` must be a valid pointer.
 */

struct GoResult jrsonnet_go_evaluate_snippet(const struct VM *vm,
                                             const char *filename,
                                             const char *snippet);

/**
 * Evaluate a Jsonnet file.
 *
 * # Safety
 *
 * `filename` must be a NUL-terminated string.
 * `vm` must be a valid pointer.
 */
 struct GoResult jrsonnet_go_evaluate_file(const struct VM *vm, const char *filename);

extern void free(void *ptr);

/**
 * Set a custom import callback on the VM. This replaces the default file import resolver.
 *
 * # Safety
 *
 * `cb` must be a valid function pointer. `ctx` is passed through as-is.
 */
 void jrsonnet_go_import_callback(const struct VM *vm, GoImportCallback cb, void *ctx);

/**
 * Register a native function on the VM.
 *
 * # Safety
 *
 * - `vm` must be a valid VM pointer
 * - `name` must be NUL-terminated
 * - `cb` must be a valid function pointer
 * - `params_json` must be a NUL-terminated JSON array of param name strings, e.g. `["x","y"]`
 */

void jrsonnet_go_native_callback(const struct VM *vm,
                                 const char *name,
                                 GoNativeCallback cb,
                                 void *ctx,
                                 const char *params_json);

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
 * Add a library search path (jpath).
 *
 * # Safety
 *
 * `path` must be a NUL-terminated string.
 */
 void jrsonnet_go_jpath_add(const struct VM *vm, const char *path);

/**
 * Bind an external string variable.
 *
 * # Safety
 *
 * `name`, `value` must be NUL-terminated strings.
 */
 int jrsonnet_go_ext_var(const struct VM *vm, const char *name, const char *value);

/**
 * Bind an external code variable. Returns 0 on success, 1 on parse error.
 * On error, writes error message to `err_out` (caller must free with `jrsonnet_go_free_string`).
 *
 * # Safety
 *
 * `name`, `code` must be NUL-terminated strings.
 * `err_out` must be a valid pointer.
 */
 int jrsonnet_go_ext_code(const struct VM *vm, const char *name, const char *code, char **err_out);

/**
 * Bind a top-level string argument.
 *
 * # Safety
 *
 * `name`, `value` must be NUL-terminated strings.
 */
 int jrsonnet_go_tla_var(struct VM *vm, const char *name, const char *value);

/**
 * Bind a top-level code argument. Returns 0 on success, 1 on error.
 *
 * # Safety
 *
 * `name`, `code` must be NUL-terminated strings.
 * `err_out` must be a valid pointer.
 */
 int jrsonnet_go_tla_code(struct VM *vm, const char *name, const char *code, char **err_out);

/**
 * Clear all TLA arguments.
 */
 void jrsonnet_go_tla_reset(struct VM *vm);

#endif  /* JRSONNET_GO_H */
