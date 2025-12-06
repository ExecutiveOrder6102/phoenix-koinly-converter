package main

import (
	"flag" // Import the flag package for command-line argument parsing
	"log"
	"os"

	"github.com/ExecutiveOrder6102/phoenix-koinly-converter/converter"
)

func main() {
	// Define command line flags.
	flag.BoolVar(&converter.Verbose, "v", false, "Enable verbose logging for debugging.")
	flag.Parse() // Parse command-line arguments.

	// Check if a file path is provided after parsing flags.
	if flag.NArg() < 1 {
		log.Fatal("Please provide the path to the Phoenix CSV file.")
	}
	filePath := flag.Arg(0) // Get the file path from the non-flag arguments.

	f, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Error opening Phoenix CSV: %v", err)
	}
	defer f.Close()

	// Create the Koinly CSV file.
	koinlyFile, err := os.Create("koinly.csv")
	if err != nil {
		log.Fatalf("Error creating Koinly CSV: %v", err)
	}
	defer koinlyFile.Close()

	if err := converter.Convert(f, koinlyFile); err != nil {
		log.Fatalf("Conversion failed: %v", err)
	}

	log.Println("Conversion complete: koinly.csv created successfully.")
}
