.PHONY: build test install clean

BIN_DIR := .bin
ROOT := ..
GO_ENV := CGO_ENABLED=0 GOCACHE=$(CURDIR)/.cache/go-build

build:
	mkdir -p $(BIN_DIR)
	cd $(ROOT) && $(GO_ENV) go build -o simple/$(BIN_DIR)/simple ./simple/runtime/cmd/simple

test:
	cd $(ROOT) && $(GO_ENV) go test ./simple/runtime/...

install: build

clean:
	rm -rf $(BIN_DIR) .cache out
