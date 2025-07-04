package main

import (
	"encoding/csv"
	"flag" // Import the flag package for command-line argument parsing
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	// koinlyDateFormat defines the date format required by Koinly CSV.
	koinlyDateFormat = "2006-01-02 15:04:05 Z"
	// phoenixDateFormat defines the date format used in Phoenix CSV exports.
	phoenixDateFormat = "2006-01-02T15:04:05.999Z"
)

// verbose enables or disables verbose logging.
var verbose bool

// KoinlyRecord represents a single row in the Koinly CSV file.
type KoinlyRecord struct {
	Date             string
	SentAmount       string
	SentCurrency     string
	ReceivedAmount   string
	ReceivedCurrency string
	FeeAmount        string
	FeeCurrency      string
	NetWorthAmount   string
	NetWorthCurrency string
	Label            string
	Description      string
	TxHash           string
}

// PhoenixRecord represents a single row in the Phoenix CSV file.
type PhoenixRecord struct {
	Timestamp       time.Time
	Type            string
	AmountMillisats int64
	MiningFeeSat    int64
	ServiceFeeMsat  int64
	TransactionID   string
	Description     string
}

func main() {
	// Define the verbose flag.
	flag.BoolVar(&verbose, "v", false, "Enable verbose logging for debugging.")
	flag.Parse() // Parse command-line arguments.

	// Check if a file path is provided after parsing flags.
	if flag.NArg() < 1 {
		log.Fatal("Please provide the path to the Phoenix CSV file.")
	}
	filePath := flag.Arg(0) // Get the file path from the non-flag arguments.

	// Read all records from the Phoenix CSV file.
	phoenixRecords, err := readPhoenixCSV(filePath)
	if err != nil {
		log.Fatalf("Error reading Phoenix CSV: %v", err)
	}

	// Create the Koinly CSV file.
	if err := createKoinlyCSV(phoenixRecords, "koinly.csv"); err != nil {
		log.Fatalf("Error creating Koinly CSV: %v", err)
	}

	log.Println("Conversion complete: koinly.csv created successfully.")
}

// logVerbose prints messages only if the verbose flag is enabled.
func logVerbose(format string, v ...interface{}) {
	if verbose {
		log.Printf(format, v...)
	}
}

// readPhoenixCSV reads a CSV file from the given path and parses it into a slice of PhoenixRecord.
func readPhoenixCSV(filePath string) ([]*PhoenixRecord, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	// Read header row to skip it.
	_, err = r.Read()
	if err != nil {
		return nil, err
	}

	var records []*PhoenixRecord
	for {
		record, err := r.Read()
		if err == io.EOF { // End of file reached.
			break
		}
		if err != nil {
			return nil, err
		}

		phoenixRecord, err := parsePhoenixRecord(record)
		if err != nil {
			// Log parsing errors but continue processing other records.
			log.Printf("Error parsing record: %v. Skipping this record.", err)
			continue
		}
		records = append(records, phoenixRecord)
	}
	return records, nil
}

// createKoinlyCSV takes a slice of PhoenixRecord and writes them to a new CSV file
// formatted for Koinly.
func createKoinlyCSV(records []*PhoenixRecord, filePath string) error {
	koinlyFile, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer koinlyFile.Close()

	w := csv.NewWriter(koinlyFile)
	defer w.Flush() // Ensure all buffered writes are committed to the underlying writer.

	// Define the header for the Koinly CSV file.
	koinlyHeader := []string{
		"Date",
		"Sent Amount",
		"Sent Currency",
		"Received Amount",
		"Received Currency",
		"Fee Amount",
		"Fee Currency",
		"Net Worth Amount",
		"Net Worth Currency",
		"Label",
		"Description",
		"TxHash",
	}
	if err := w.Write(koinlyHeader); err != nil {
		return err
	}

	// Convert each Phoenix record to a Koinly record and write it to the CSV.
	for _, p := range records {
		koinlyRecord := toKoinlyRecord(p)
		if err := w.Write(koinlyRecord.toStringSlice()); err != nil {
			return err
		}
	}
	return nil
}

// parsePhoenixRecord converts a slice of strings (a row from Phoenix CSV) into a PhoenixRecord struct.
func parsePhoenixRecord(record []string) (*PhoenixRecord, error) {
	// Parse timestamp.
	timestamp, err := time.Parse(phoenixDateFormat, record[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp '%s': %w", record[0], err)
	}

	// Parse amount in millisats, handling potential commas.
	amountMillisats, err := strconv.ParseInt(strings.ReplaceAll(record[3], ",", ""), 10, 64)
	if err != nil {
		// If parsing fails, default to 0 and log the error.
		logVerbose("Warning: Failed to parse AmountMillisats '%s'. Defaulting to 0. Error: %v", record[3], err)
		amountMillisats = 0
	}

	// Parse mining fee in sats, handling potential commas.
	miningFeeSat, err := strconv.ParseInt(strings.ReplaceAll(record[6], ",", ""), 10, 64)
	if err != nil {
		logVerbose("Warning: Failed to parse MiningFeeSat '%s'. Defaulting to 0. Error: %v", record[6], err)
		miningFeeSat = 0
	}

	// Parse service fee in millisats, handling potential commas.
	serviceFeeMsat, err := strconv.ParseInt(strings.ReplaceAll(record[8], ",", ""), 10, 64)
	if err != nil {
		logVerbose("Warning: Failed to parse ServiceFeeMsat '%s'. Defaulting to 0. Error: %v", record[8], err)
		serviceFeeMsat = 0
	}

	return &PhoenixRecord{
		Timestamp:       timestamp,
		Type:            record[2],
		AmountMillisats: amountMillisats,
		MiningFeeSat:    miningFeeSat,
		ServiceFeeMsat:  serviceFeeMsat,
		TransactionID:   record[11],
		Description:     record[13],
	}, nil
}

// toKoinlyRecord converts a PhoenixRecord into a KoinlyRecord.
// It maps different Phoenix transaction types to appropriate Koinly fields (Sent, Received, Fee).
func toKoinlyRecord(p *PhoenixRecord) *KoinlyRecord {
	// Note: Fees are often included in the sent/received amounts in Phoenix,
	// so they are not always tracked separately in Koinly unless explicitly a fee-only transaction.
	k := &KoinlyRecord{
		Date:        p.Timestamp.Format(koinlyDateFormat),
		TxHash:      p.TransactionID,
		Description: p.Description,
	}

	// Convert amount from millisats to sats.
	sats := float64(p.AmountMillisats) / 1000

	logVerbose("Processing Phoenix Record: %+v", p)
	logVerbose("Calculated Sats: %.8f", sats)

	// Determine the Koinly record type based on Phoenix transaction type.
	switch p.Type {
	case "lightning_received":
		k.ReceivedAmount = fmt.Sprintf("%.8f", sats/100000000) // Convert sats to BTC.
		k.ReceivedCurrency = "BTC"
		k.Label = "lightning"
		logVerbose("Type: lightning_received -> ReceivedAmount=%s BTC", k.ReceivedAmount)
	case "lightning_sent":
		// For sent transactions, amount_msat is negative. Use absolute value.
		k.SentAmount = fmt.Sprintf("%.8f", math.Abs(sats)/100000000)
		k.SentCurrency = "BTC"
		k.Label = "lightning"
		logVerbose("Type: lightning_sent -> SentAmount=%s BTC", k.SentAmount)
	case "swap_in", "legacy_swap_in":
		// Swap-in is a receipt of funds.
		k.ReceivedAmount = fmt.Sprintf("%.8f", sats/100000000)
		k.ReceivedCurrency = "BTC"
		k.Label = "transfer"
		logVerbose("Type: %s -> ReceivedAmount=%s BTC", p.Type, k.ReceivedAmount)
	case "swap_out":
		// Swap-out is a sending of funds.
		k.SentAmount = fmt.Sprintf("%.8f", math.Abs(sats)/100000000)
		k.SentCurrency = "BTC"
		k.Label = "transfer"
		logVerbose("Type: swap_out -> SentAmount=%s BTC", k.SentAmount)
	case "channel_open", "legacy_pay_to_open":
		// Channel open is treated as a deposit.
		k.ReceivedAmount = fmt.Sprintf("%.8f", sats/100000000)
		k.ReceivedCurrency = "BTC"
		k.Label = "deposit"
		logVerbose("Type: %s -> ReceivedAmount=%s BTC", p.Type, k.ReceivedAmount)
	case "channel_close":
		// Channel close is treated as a cost (fee) in Koinly, as it's often just a fee settlement.
		k.SentAmount = ""
		k.SentCurrency = ""
		k.ReceivedAmount = ""
		k.ReceivedCurrency = ""
		k.FeeAmount = fmt.Sprintf("%.8f", math.Abs(sats)/100000000)
		k.FeeCurrency = "BTC"
		k.Label = "cost"
		logVerbose("Type: channel_close -> FeeAmount=%s BTC", k.FeeAmount)
	default:
		// Log unknown transaction types for awareness.
		log.Printf("Unknown transaction type for Koinly conversion: %s. This transaction will not be fully converted.", p.Type)
	}

	return k
}

// toStringSlice converts a KoinlyRecord struct into a slice of strings,
// suitable for writing as a row in a CSV file.
func (k *KoinlyRecord) toStringSlice() []string {
	return []string{
		k.Date,
		k.SentAmount,
		k.SentCurrency,
		k.ReceivedAmount,
		k.ReceivedCurrency,
		k.FeeAmount,
		k.FeeCurrency,
		k.NetWorthAmount,
		k.NetWorthCurrency,
		k.Label,
		k.Description,
		k.TxHash,
	}
}
