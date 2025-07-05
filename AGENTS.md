# Agent Guidelines

This repository contains a Go command line utility that converts Phoenix Bitcoin Lightning wallet CSV exports into a format compatible with Koinly.

## Coding Guidelines
- Format all Go files using `gofmt -w` before committing.
- Keep functions small and focused. Prefer straightforward loops and clear variable names.
- Use the existing structs and helper functions when extending functionality.

## Testing Guidelines
- Always run `go test ./...` before committing changes. All tests must pass.
- If you add new dependencies, run `go mod tidy` to update `go.mod` and `go.sum`.

## Repository Structure
- `main.go` contains the conversion logic and CLI.
- `main_test.go` contains unit tests for parsing and conversion.
- `testdata/` holds sample CSV files used in tests.

Follow these guidelines to maintain code consistency and reliability.
