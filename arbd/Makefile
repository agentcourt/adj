.DEFAULT_GOAL := build

.PHONY: aardengine build test prove install demo clean

BIN_DIR := .bin
GO_ENV := CGO_ENABLED=0 GOCACHE=$(CURDIR)/.cache/go-build GOMAXPROCS=2 LEAN_NUM_THREADS=1

aardengine:
	mkdir -p $(BIN_DIR)
	cd engine && LEAN_NUM_THREADS=1 lake build aardengine
	install -m 755 engine/.lake/build/bin/aardengine $(BIN_DIR)/aardengine

build: aardengine
	cd runtime && $(GO_ENV) go build -o ../$(BIN_DIR)/aard ./cmd/aard
	cd runtime && $(GO_ENV) go build -o ../$(BIN_DIR)/aard-mcp ./cmd/aard-mcp
	cd .. && $(GO_ENV) go build -buildvcs=false -o arbd/$(BIN_DIR)/aard-run ./cmd/aard-run

test: build
	cd runtime && $(GO_ENV) go test -p 1 -parallel 1 ./...
	cd .. && $(GO_ENV) go test -p 1 -parallel 1 ./runtime/localrun/arbd ./cmd/aard-run

prove:
	cd engine && LEAN_NUM_THREADS=1 lake build Proofs

install: build

demo: build
	@echo ".bin/aard case --complaint examples/ex1/complaint.md --out-dir out/ex1 --caseapi-addr 127.0.0.1:19791"

clean:
	rm -rf $(BIN_DIR) .cache engine/.lake/build out
