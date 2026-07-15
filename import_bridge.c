#include <stddef.h>
#include <stdint.h>

// Defined in import.go via //export goImportCallback
extern int goImportCallback(uintptr_t ctx, const char *base, const char *rel,
                            char **found_here, char **buf, uintptr_t *buflen);

int goImportCallbackBridge(uintptr_t ctx, const char *base, const char *rel,
                           char **found_here, char **buf, uintptr_t *buflen) {
    return goImportCallback(ctx, base, rel, found_here, buf, buflen);
}
