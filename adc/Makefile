.PHONY: build test prove install demo demo-quick clean

BIN_DIR := .bin
GO_ENV := CGO_ENABLED=0

build:
	mkdir -p $(BIN_DIR)
	cd engine && lake build adcengine
	install -m 755 engine/.lake/build/bin/adcengine $(BIN_DIR)/adcengine
	cd runtime && $(GO_ENV) go build -o ../$(BIN_DIR)/adc ./cmd/adc

test:
	cd runtime && $(GO_ENV) go test ./...

prove:
	cd engine && lake build Proofs

install: build

demo: build
	rm -rf out/ex1-demo
	examples/ex1/sign.sh
	.bin/adc complain --situation examples/ex1/situation.md --out examples/ex1/complaint.md
	.bin/adc case \
		--complaint examples/ex1/complaint.md \
		--out-dir out/ex1-demo

demo-quick: build
	rm -rf out/ex1-demo-quick
	examples/ex1/sign.sh
	.bin/adc complain --situation examples/ex1/situation.md --out examples/ex1/complaint.md
	.bin/adc case \
		--complaint examples/ex1/complaint.md \
		--out-dir out/ex1-demo-quick \
		--skip-voir-dire

clean:
	rm -rf $(BIN_DIR) engine/.lake/build
