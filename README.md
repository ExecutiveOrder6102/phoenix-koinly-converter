# Phoenix Koinly Converter

This project converts transaction data exported from the Phoenix Bitcoin Lightning wallet into a CSV format compatible with Koinly, cryptocurrency tax software. It includes both a traditional command-line workflow and an in-browser experience powered by WebAssembly.

## Features

- Converts Phoenix transaction types (lightning_received, lightning_sent, swap_in, swap_out, channel_open, channel_close) into Koinly-compatible records.
- Handles amount conversions from millisats to BTC.
- CLI mode for converting CSV exports on your machine.
- In-browser converter (WASM) that runs entirely client-side with drag-and-drop upload and a downloadable `koinly.csv` output.
- Verbose logging flag for additional insight when debugging CLI runs.

## Usage

### Command-line converter

Provide the path to your Phoenix CSV export file as a command-line argument. The converter writes a `koinly.csv` file to the current working directory.

```bash
go run main.go <path_to_phoenix_csv_file>
```

**Verbose mode example:**

```bash
go run main.go -v phoenix_transactions.csv
```

This generates `koinly.csv`, which you can import into Koinly.

### In-browser converter (WASM)

The `web/` directory contains a drag-and-drop interface that converts Phoenix exports entirely in your browser. Build the WebAssembly bundle and serve the static files locally:

```bash
make build-wasm
cd web
python -m http.server 8000
```

Open http://localhost:8000 in your browser, drop your Phoenix CSV export, and download the generated `koinly.csv` file. No data leaves your device.

### Verbose Logging

To enable verbose logging for detailed debugging output, use the `-v` flag:

```bash
go run main.go -v <path_to_phoenix_csv_file>
```

## Building from Source

Use the provided `Makefile` to build the CLI binary and the WASM bundle.

```bash
make build-cli   # Builds the CLI binary at ./phoenix-koinly-converter
make build-wasm  # Produces web/main.wasm and copies wasm_exec.js into web/
```

Then you can run the executable:

```bash
./phoenix-koinly-converter <path_to_phoenix_csv_file>
```

## Supported Phoenix Transaction Types

- `lightning_received`: Treated as received BTC.
- `lightning_sent`: Treated as sent BTC.
- `swap_in` / `legacy_swap_in`: Treated as received BTC (transfers).
- `swap_out`: Treated as sent BTC (transfers).
- `channel_open` / `legacy_pay_to_open`: Treated as received BTC (deposits).
- `channel_close`: Treated as a fee/cost in BTC.

Any other transaction types will be logged as unknown and may not be fully converted.