.PHONY: build test prove install demo clean

BIN_DIR := .bin
GO_ENV := CGO_ENABLED=0 GOCACHE=$(CURDIR)/.cache/go-build

build:
	mkdir -p $(BIN_DIR)
	cd engine && lake build aardengine
	install -m 755 engine/.lake/build/bin/aardengine $(BIN_DIR)/aardengine
	cd runtime && $(GO_ENV) go build -o ../$(BIN_DIR)/aard ./cmd/aard

test:
	cd runtime && $(GO_ENV) go test ./...

prove:
	cd engine && lake build Proofs

install: build

demo: build
	@echo ".bin/aard case --complaint examples/ex1/complaint.md --out-dir out/ex1 --caseapi-addr 127.0.0.1:19791"

clean:
	rm -rf $(BIN_DIR) .cache engine/.lake/build out
