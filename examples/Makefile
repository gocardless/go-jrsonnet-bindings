# Minimal build wiring for a go-jrsonnet-bindings consumer.
#
# The static library is not in git: it is fetched (fetch-lib.sh) for the target
# triple, then the linker is pointed at it via CGO_LDFLAGS. Adapt freely — this
# is a template, not a drop-in.

GOOS   := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

# gnu (dynamic glibc) or musl (fully static). Ignored on darwin.
LIBC ?= gnu

# GOOS/GOARCH(/LIBC on linux) -> Rust target triple.
TRIPLE_darwin_arm64     := aarch64-apple-darwin
TRIPLE_linux_amd64_gnu  := x86_64-unknown-linux-gnu
TRIPLE_linux_arm64_gnu  := aarch64-unknown-linux-gnu
TRIPLE_linux_amd64_musl := x86_64-unknown-linux-musl
TRIPLE_linux_arm64_musl := aarch64-unknown-linux-musl
ifeq ($(GOOS),darwin)
TRIPLE := $(TRIPLE_$(GOOS)_$(GOARCH))
else
TRIPLE := $(TRIPLE_$(GOOS)_$(GOARCH)_$(LIBC))
endif

# Pin this to the go-jrsonnet-bindings version your go.mod requires. Here we
# read it back from the module graph so the fetched lib matches the source.
VERSION = $(shell go list -m -f '{{.Version}}' github.com/gocardless/go-jrsonnet-bindings)

# Downloaded libs land here. These are build artifacts, not source: add
# .jrsonnet/ to your .gitignore.
LIB_ROOT := $(CURDIR)/.jrsonnet
LIB_DIR  := $(LIB_ROOT)/lib/$(TRIPLE)
LIB      := $(LIB_DIR)/libjrsonnet_go.a

# jrsonnet requires CGO, and the linker finds -ljrsonnet_go via this -L path.
export CGO_ENABLED := 1
export CGO_LDFLAGS := -L$(LIB_DIR)

# musl builds must link fully static AND bump the thread stack size: jrsonnet
# recurses deeply and musl's small default stack segfaults without it. These
# only apply to linux+musl; gnu/darwin stay dynamic with the runtime default.
ifeq ($(GOOS)-$(LIBC),linux-musl)
LDFLAGS := -linkmode external -extldflags "-static -Wl,-z,stack-size=8388608"
endif

# Swap in fetch-lib-gh.sh here if you prefer the gh CLI over curl.
$(LIB):
	@test -n "$(TRIPLE)" || { echo "unsupported platform $(GOOS)/$(GOARCH)"; exit 1; }
	./fetch-lib.sh "$(VERSION)" "$(TRIPLE)" "$(LIB_ROOT)"

.PHONY: build
build: $(LIB)
	go build $(if $(LDFLAGS),-ldflags '$(LDFLAGS)') -o bin/example .

.PHONY: clean
clean:
	rm -rf bin $(LIB_ROOT)
