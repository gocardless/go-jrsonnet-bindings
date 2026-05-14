INCLUDE_DIR := include
CBINDGEN := $(HOME)/.cargo/bin/cbindgen

.PHONY: rust header test clean

rust:
	bash scripts/build-rust.sh

header:
	$(CBINDGEN) --crate jrsonnet-go --output $(INCLUDE_DIR)/jrsonnet_go.h

test: rust
	go test -v -race -count=1 ./...

clean:
	cargo clean
	rm -f $(INCLUDE_DIR)/jrsonnet_go.h
	rm -rf lib
