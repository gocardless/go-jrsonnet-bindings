#include <stddef.h>
#include <stdint.h>

// Defined in import.go via //export goImportCallback
extern int goImportCallback(uintptr_t ctx, const char *base, uintptr_t base_len,
                            const char *rel, uintptr_t rel_len,
                            char **found_here, uintptr_t *found_here_len,
                            char **buf, uintptr_t *buf_len);

int goImportCallbackBridge(uintptr_t ctx, const char *base, uintptr_t base_len,
                           const char *rel, uintptr_t rel_len,
                           char **found_here, uintptr_t *found_here_len,
                           char **buf, uintptr_t *buf_len) {
    return goImportCallback(ctx, base, base_len, rel, rel_len, found_here,
                            found_here_len, buf, buf_len);
}
