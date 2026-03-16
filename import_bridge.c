#include <stddef.h>
#include <stdint.h>

// Defined in import.go via //export goImportCallback
extern int goImportCallback(void *ctx, const char *base, const char *rel,
                            char **found_here, char **buf, uintptr_t *buflen);

int goImportCallbackBridge(void *ctx, const char *base, const char *rel,
                           char **found_here, char **buf, uintptr_t *buflen) {
    return goImportCallback(ctx, base, rel, found_here, buf, buflen);
}
