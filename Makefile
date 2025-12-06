.PHONY: all build-cli build-wasm clean

all: build-cli build-wasm

build-cli:
	go build -o phoenix-koinly-converter main.go

build-wasm:
	cp $$(find $$(go env GOROOT) -name wasm_exec.js | head -n 1) web/
	GOOS=js GOARCH=wasm go build -o web/main.wasm cmd/wasm/main.go

clean:
	rm -f phoenix-koinly-converter web/main.wasm koinly.csv
