INCLUDE_DIR := include
CBINDGEN := $(HOME)/.cargo/bin/cbindgen

.PHONY: rust rust-all header test clean

rust:
	bash scripts/build-rust.sh

rust-all:
	bash scripts/build-rust.sh all

header:
	$(CBINDGEN) --crate jrsonnet-go --output $(INCLUDE_DIR)/jrsonnet_go.h

test: rust
	cargo test --lib
	go test -v -race -count=1 ./...
	cd conformance && go test -count=1 ./...

clean:
	cargo clean
	rm -f $(INCLUDE_DIR)/jrsonnet_go.h
	rm -rf lib
