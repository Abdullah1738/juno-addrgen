.PHONY: build rust-build rust-test test test-unit test-integration test-e2e clean

BIN_DIR := bin
BIN := $(BIN_DIR)/juno-addrgen

RUST_MANIFEST := rust/addrgen/Cargo.toml

build: rust-build
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN) ./cmd/juno-addrgen

rust-build:
	cargo build --release --manifest-path $(RUST_MANIFEST)

rust-test:
	cargo test --manifest-path $(RUST_MANIFEST)

test-unit: rust-test
	CGO_ENABLED=0 go test ./internal/cli

test-integration: rust-build
	go test ./pkg/addrgen

test-e2e: build
	go test -tags=e2e ./internal/e2e

test: test-unit test-integration test-e2e

clean:
	rm -rf $(BIN_DIR)
	rm -rf rust/addrgen/target
