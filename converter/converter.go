package converter

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	// KoinlyDateFormat defines the date format required by Koinly CSV.
	KoinlyDateFormat = "2006-01-02 15:04:05 Z"
	// PhoenixDateFormat defines the date format used in Phoenix CSV exports.
	PhoenixDateFormat = "2006-01-02T15:04:05.999Z"
	satsPerBTC        = 100000000
	msatsPerSat       = 1000
)

var (
	Verbose bool
)

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

// ParseIntField parses an integer field with commas.
func ParseIntField(val, name string) int64 {
	v, err := strconv.ParseInt(strings.ReplaceAll(val, ",", ""), 10, 64)
	if err != nil {
		LogVerbose("Warning: failed to parse %s '%s': %v", name, val, err)
		return 0
	}
	return v
}

// FormatBTC formats sats to a BTC string.
func FormatBTC(sats float64) string {
	return fmt.Sprintf("%.8f", sats/satsPerBTC)
}

// LogVerbose prints messages only if the verbose flag is enabled.
func LogVerbose(format string, v ...interface{}) {
	if Verbose {
		log.Printf(format, v...)
	}
}

// Convert handles the core conversion logic from a reader to a writer.
func Convert(r io.Reader, w io.Writer, addRoundingCost bool) error {
	phoenixRecords, err := ReadPhoenixCSV(r)
	if err != nil {
		return fmt.Errorf("reading phoenix csv: %w", err)
	}

	if err := CreateKoinlyCSV(phoenixRecords, w, addRoundingCost); err != nil {
		return fmt.Errorf("creating koinly csv: %w", err)
	}
	return nil
}

// ReadPhoenixCSV reads a CSV file from the given reader and parses it into a slice of PhoenixRecord.
func ReadPhoenixCSV(r io.Reader) ([]*PhoenixRecord, error) {
	reader := csv.NewReader(r)
	// Read header row to skip it.
	_, err := reader.Read()
	if err != nil {
		return nil, err
	}

	var records []*PhoenixRecord
	for {
		record, err := reader.Read()
		if err == io.EOF { // End of file reached.
			break
		}
		if err != nil {
			return nil, err
		}

		phoenixRecord, err := ParsePhoenixRecord(record)
		if err != nil {
			// Log parsing errors but continue processing other records.
			log.Printf("Error parsing record: %v. Skipping this record.", err)
			continue
		}
		records = append(records, phoenixRecord)
	}
	return records, nil
}

// CreateKoinlyCSV takes a slice of PhoenixRecord and writes them to a new CSV file
// formatted for Koinly.
func CreateKoinlyCSV(records []*PhoenixRecord, w io.Writer, addCost bool) error {
	writer := csv.NewWriter(w)
	defer writer.Flush() // Ensure all buffered writes are committed to the underlying writer.

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
	if err := writer.Write(koinlyHeader); err != nil {
		return err
	}

	var roundingDiff float64
	// Convert each Phoenix record to a Koinly record and write it to the CSV.
	for _, p := range records {
		koinlyRecord, diff := ToKoinlyRecord(p)
		roundingDiff += diff
		if err := writer.Write(koinlyRecord.ToStringSlice()); err != nil {
			return err
		}
	}

	if addCost {
		roundingSats := int64(math.Round(math.Abs(roundingDiff)))
		if roundingSats > 0 {
			costRecord := &KoinlyRecord{
				Date:        time.Now().UTC().Format(KoinlyDateFormat),
				FeeAmount:   FormatBTC(float64(roundingSats)),
				FeeCurrency: "BTC",
				Label:       "cost",
				Description: "Adjustment for rounding differences",
			}
			if err := writer.Write(costRecord.ToStringSlice()); err != nil {
				return err
			}
		}
	}
	return nil
}

// ParsePhoenixRecord converts a slice of strings (a row from Phoenix CSV) into a PhoenixRecord struct.
func ParsePhoenixRecord(record []string) (*PhoenixRecord, error) {
	// Parse timestamp.
	timestamp, err := time.Parse(PhoenixDateFormat, record[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp '%s': %w", record[0], err)
	}

	amountMillisats := ParseIntField(record[3], "amount_msat")
	miningFeeSat := ParseIntField(record[6], "mining_fee_sat")
	serviceFeeMsat := ParseIntField(record[8], "service_fee_msat")

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

// ToKoinlyRecord converts a PhoenixRecord into a KoinlyRecord.
// It maps different Phoenix transaction types to appropriate Koinly fields (Sent, Received, Fee).
func ToKoinlyRecord(p *PhoenixRecord) (*KoinlyRecord, float64) {
	// Note: Fees are often included in the sent/received amounts in Phoenix,
	// so they are not always tracked separately in Koinly unless explicitly a fee-only transaction.
	k := &KoinlyRecord{
		Date:        p.Timestamp.Format(KoinlyDateFormat),
		TxHash:      p.TransactionID,
		Description: p.Description,
	}

	// Convert amount from millisats to sats.
	sats := float64(p.AmountMillisats) / msatsPerSat
	absSats := math.Abs(sats)
	LogVerbose("Processing Phoenix Record: %+v", p)
	LogVerbose("Calculated Sats: %.8f", sats)

	var diff float64
	// Determine the Koinly record type based on Phoenix transaction type.
	switch p.Type {
	case "lightning_received":
		amt := FormatBTC(sats)
		k.ReceivedAmount = amt
		k.ReceivedCurrency = "BTC"
		k.Label = "lightning"
		LogVerbose("Type: lightning_received -> ReceivedAmount=%s BTC", k.ReceivedAmount)
		v, _ := strconv.ParseFloat(amt, 64)
		diff = sats - v*satsPerBTC
	case "lightning_sent":
		// For sent transactions, amount_msat is negative. Use absolute value.
		amt := FormatBTC(absSats)
		k.SentAmount = amt
		k.SentCurrency = "BTC"
		k.Label = "lightning"
		LogVerbose("Type: lightning_sent -> SentAmount=%s BTC", k.SentAmount)
		v, _ := strconv.ParseFloat(amt, 64)
		diff = sats - (-v * satsPerBTC)
	case "swap_in", "legacy_swap_in":
		// Swap-in is a receipt of funds.
		amt := FormatBTC(sats)
		k.ReceivedAmount = amt
		k.ReceivedCurrency = "BTC"
		k.Label = "transfer"
		LogVerbose("Type: %s -> ReceivedAmount=%s BTC", p.Type, k.ReceivedAmount)
		v, _ := strconv.ParseFloat(amt, 64)
		diff = sats - v*satsPerBTC
	case "swap_out":
		// Swap-out is a sending of funds.
		amt := FormatBTC(absSats)
		k.SentAmount = amt
		k.SentCurrency = "BTC"
		k.Label = "transfer"
		LogVerbose("Type: swap_out -> SentAmount=%s BTC", k.SentAmount)
		v, _ := strconv.ParseFloat(amt, 64)
		diff = sats - (-v * satsPerBTC)
	case "channel_open", "legacy_pay_to_open":
		// Channel open is treated as a deposit.
		amt := FormatBTC(sats)
		k.ReceivedAmount = amt
		k.ReceivedCurrency = "BTC"
		k.Label = "deposit"
		LogVerbose("Type: %s -> ReceivedAmount=%s BTC", p.Type, k.ReceivedAmount)
		v, _ := strconv.ParseFloat(amt, 64)
		diff = sats - v*satsPerBTC
	case "channel_close":
		// Channel close is treated as a cost (fee) in Koinly, as it's often just a fee settlement.
		k.SentAmount = ""
		k.SentCurrency = ""
		k.ReceivedAmount = ""
		k.ReceivedCurrency = ""
		amt := FormatBTC(absSats)
		k.FeeAmount = amt
		k.FeeCurrency = "BTC"
		k.Label = "cost"
		LogVerbose("Type: channel_close -> FeeAmount=%s BTC", k.FeeAmount)
		v, _ := strconv.ParseFloat(amt, 64)
		diff = sats - (-v * satsPerBTC)
	default:
		// Log unknown transaction types for awareness.
		log.Printf("Unknown transaction type for Koinly conversion: %s. This transaction will not be fully converted.", p.Type)
	}

	return k, diff
}

// ToStringSlice converts a KoinlyRecord struct into a slice of strings,
// suitable for writing as a row in a CSV file.
func (k *KoinlyRecord) ToStringSlice() []string {
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
