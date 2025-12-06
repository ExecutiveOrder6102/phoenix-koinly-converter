.PHONY: all build-cli build-wasm clean

all: build-cli build-wasm

build-cli:
	go build -o phoenix-koinly-converter main.go

build-wasm:
	GOOS=js GOARCH=wasm go build -o web/main.wasm cmd/wasm/main.go

clean:
	rm -f phoenix-koinly-converter web/main.wasm koinly.csv
