.DEFAULT_GOAL := build

.PHONY: build procedures runners test model-pool

build: procedures runners

procedures:
	$(MAKE) -C simple build
	$(MAKE) -C quick build
	$(MAKE) -C arb build
	$(MAKE) -C arbd build
	$(MAKE) -C adc build

runners:
	mkdir -p .bin
	CGO_ENABLED=0 go build -buildvcs=false -o .bin/adjudicate ./cmd/adjudicate
	CGO_ENABLED=0 go build -buildvcs=false -o .bin/model-config-screen ./cmd/model-config-screen

test:
	go test ./...

model-pool:
	test -n "$(strip $(RUN_ID))" || { echo "RUN_ID is required" >&2; exit 2; }
	$(MAKE) -C model-pool pool RUN_ID="$(RUN_ID)"
