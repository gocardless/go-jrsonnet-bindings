RUST_DIR := rust
INCLUDE_DIR := include
CBINDGEN := $(HOME)/.cargo/bin/cbindgen

.PHONY: rust header test clean

rust:
	bash scripts/build-rust.sh

header:
	cd $(RUST_DIR) && RUSTUP_TOOLCHAIN=stable $(CBINDGEN) --config cbindgen.toml --output ../$(INCLUDE_DIR)/jrsonnet_go.h

test: rust
	go test -v -race -count=1 ./...

clean:
	cd $(RUST_DIR) && cargo clean
	rm -f $(INCLUDE_DIR)/jrsonnet_go.h
	rm -rf lib/darwin-arm64/*.a lib/linux-amd64/*.a
