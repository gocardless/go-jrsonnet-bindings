#include <stddef.h>
#include <stdint.h>

// Defined in native.go via //export goNativeCallback
extern int goNativeCallback(uintptr_t ctx, const char *args_json,
                            uintptr_t args_json_len, char **result_json,
                            uintptr_t *result_json_len);

int goNativeCallbackBridge(uintptr_t ctx, const char *args_json,
                           uintptr_t args_json_len, char **result_json,
                           uintptr_t *result_json_len) {
    return goNativeCallback(ctx, args_json, args_json_len, result_json,
                            result_json_len);
}
