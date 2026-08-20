.DEFAULT_GOAL := build

.PHONY: aarengine build test prove install demo clean

BIN_DIR := .bin
GO_ENV := CGO_ENABLED=0 GOCACHE=$(CURDIR)/.cache/go-build

aarengine:
	mkdir -p $(BIN_DIR)
	cd engine && LEAN_NUM_THREADS=1 lake build aarengine
	install -m 755 engine/.lake/build/bin/aarengine $(BIN_DIR)/aarengine

build: aarengine
	cd runtime && $(GO_ENV) go build -o ../$(BIN_DIR)/aar ./cmd/aar

test: aarengine
	cd runtime && $(GO_ENV) go test ./...

prove:
	cd engine && LEAN_NUM_THREADS=1 lake build Proofs

install: build

demo: build
	@echo ".bin/aar case --complaint examples/ex01/complaint.md --out-dir out/ex01 --caseapi-addr 127.0.0.1:19771"

clean:
	rm -rf $(BIN_DIR) engine/.lake/build out
