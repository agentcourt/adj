.PHONY: build test prove install demo clean

BIN_DIR := .bin
GO_ENV := CGO_ENABLED=0 GOCACHE=$(CURDIR)/.cache/go-build

build:
	mkdir -p $(BIN_DIR)
	cd engine && lake build aarengine
	install -m 755 engine/.lake/build/bin/aarengine $(BIN_DIR)/aarengine
	cd runtime && $(GO_ENV) go build -o ../$(BIN_DIR)/aar ./cmd/aar

test:
	cd runtime && $(GO_ENV) go test ./...

prove:
	cd engine && lake build Proofs

install: build

demo: build
	@echo ".bin/aar case --complaint examples/ex01/complaint.md --out-dir out/ex01 --caseapi-addr 127.0.0.1:19771"

clean:
	rm -rf $(BIN_DIR) engine/.lake/build out
