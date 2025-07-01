# Phoenix Koinly Converter

This Go program converts transaction data exported from the Phoenix Bitcoin Lightning wallet into a CSV format compatible with Koinly, cryptocurrency tax software.

## Features

- Converts Phoenix transaction types (lightning_received, lightning_sent, swap_in, swap_out, channel_open, channel_close) into Koinly-compatible records.
- Handles amount conversions from millisats to BTC.
- Provides verbose logging for debugging purposes.

## Usage

To use the converter, you need to provide the path to your Phoenix CSV export file as a command-line argument.

```bash
go run main.go <path_to_phoenix_csv_file>
```

**Example:**

```bash
go run main.go phoenix_transactions.csv
```

This will generate a `koinly.csv` file in the same directory, which you can then import into Koinly.

### Verbose Logging

To enable verbose logging for detailed debugging output, use the `-v` flag:

```bash
go run main.go -v <path_to_phoenix_csv_file>
```

## Building from Source

To build the executable, navigate to the project directory and run:

```bash
go build -o phoenix-koinly-converter
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