.DEFAULT_GOAL := build

.PHONY: build test install clean

BIN_DIR := .bin
ROOT := ..
GO_ENV := CGO_ENABLED=0 GOCACHE=$(CURDIR)/.cache/go-build

build:
	mkdir -p $(BIN_DIR)
	cd $(ROOT) && $(GO_ENV) go build -o quick/$(BIN_DIR)/quick ./quick/cmd/quick
	cd $(ROOT) && $(GO_ENV) go build -o quick/$(BIN_DIR)/quick-mcp ./quick/cmd/quick-mcp

test:
	cd $(ROOT) && $(GO_ENV) go test ./quick/...

install: build

clean:
	rm -rf $(BIN_DIR) .cache
