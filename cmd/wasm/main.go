//go:build js && wasm

package main

import (
	"bytes"
	"fmt"
	"strings"
	"syscall/js"

	"github.com/ExecutiveOrder6102/phoenix-koinly-converter/converter"
)

func convertPhoenixToKoinly(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return "Error: No CSV data provided"
	}
	inputCSV := args[0].String()

	addRoundingCost := false
	if len(args) > 1 && args[1].Truthy() {
		addRoundingCost = args[1].Bool()
	}

	r := strings.NewReader(inputCSV)
	var buf bytes.Buffer

	// Enable verbose if needed, though we don't capture logs here easily unless we redirect log output.
	// converter.Verbose = true

	if err := converter.Convert(r, &buf, addRoundingCost); err != nil {
		return fmt.Sprintf("Error converting: %v", err)
	}

	return buf.String()
}

func main() {
	c := make(chan struct{}, 0)
	js.Global().Set("convertPhoenixToKoinly", js.FuncOf(convertPhoenixToKoinly))
	fmt.Println("WASM Initialized: convertPhoenixToKoinly function is ready.")
	<-c
}
