package main

import (
	"encoding/csv"
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
	koinlyDateFormat  = "2006-01-02 15:04:05 Z"
	phoenixDateFormat = "2006-01-02T15:04:05.999Z"
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
	Timestamp      time.Time
	Type           string
	AmountMillisats int64
	MiningFeeSat   int64
	ServiceFeeMsat int64
	TransactionID  string
	Description    string
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Please provide the path to the Phoenix CSV file.")
	}
	filePath := os.Args[1]

	// Read all records from the Phoenix CSV file.
	phoenixRecords, err := readPhoenixCSV(filePath)
	if err != nil {
		log.Fatalf("Error reading Phoenix CSV: %v", err)
	}

	// Create the Koinly CSV file.
	if err := createKoinlyCSV(phoenixRecords, "koinly.csv"); err != nil {
		log.Fatalf("Error creating Koinly CSV: %v", err)
	}

	
}

func readPhoenixCSV(filePath string) ([]*PhoenixRecord, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	// Read header
	_, err = r.Read()
	if err != nil {
		return nil, err
	}

	var records []*PhoenixRecord
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		phoenixRecord, err := parsePhoenixRecord(record)
		if err != nil {
			log.Printf("Error parsing record: %v", err)
			continue
		}
		records = append(records, phoenixRecord)
	}
	return records, nil
}

func createKoinlyCSV(records []*PhoenixRecord, filePath string) error {
	koinlyFile, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer koinlyFile.Close()

	w := csv.NewWriter(koinlyFile)
	defer w.Flush()

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

	for _, p := range records {
		koinlyRecord := toKoinlyRecord(p)
		if err := w.Write(koinlyRecord.toStringSlice()); err != nil {
			return err
		}
	}
	return nil
}

func parsePhoenixRecord(record []string) (*PhoenixRecord, error) {
	timestamp, err := time.Parse(phoenixDateFormat, record[0])
	if err != nil {
		return nil, err
	}

	amountMillisats, err := strconv.ParseInt(strings.ReplaceAll(record[3], ",", ""), 10, 64)
	if err != nil {
		amountMillisats = 0
	}

	miningFeeSat, err := strconv.ParseInt(strings.ReplaceAll(record[6], ",", ""), 10, 64)
	if err != nil {
		miningFeeSat = 0
	}

	serviceFeeMsat, err := strconv.ParseInt(strings.ReplaceAll(record[8], ",", ""), 10, 64)
	if err != nil {
		serviceFeeMsat = 0
	}

	return &PhoenixRecord{
		Timestamp:      timestamp,
		Type:           record[2],
		AmountMillisats: amountMillisats,
		MiningFeeSat:   miningFeeSat,
		ServiceFeeMsat: serviceFeeMsat,
		TransactionID:  record[11],
		Description:    record[13],
	}, nil
}

	func toKoinlyRecord(p *PhoenixRecord) *KoinlyRecord {
	// Note: Fees are included in the sent/received amounts and not tracked separately.
	// This is because Phoenix transactions often bundle fees into the total amount.
	k := &KoinlyRecord{
		Date:        p.Timestamp.Format(koinlyDateFormat),
		TxHash:      p.TransactionID,
		Description: p.Description,
	}

	sats := float64(p.AmountMillisats) / 1000

	log.Printf("Phoenix Record: %+v", p)
	log.Printf("Calculated: sats=%.8f", sats)

	switch p.Type {
	case "lightning_received":
		k.ReceivedAmount = fmt.Sprintf("%.8f", sats/100000000)
		k.ReceivedCurrency = "BTC"
		k.Label = "lightning"
		log.Printf("lightning_received: ReceivedAmount=%s", k.ReceivedAmount)
	case "lightning_sent":
		// amount_msat is negative and includes the service fee.
		k.SentAmount = fmt.Sprintf("%.8f", math.Abs(sats)/100000000)
		k.SentCurrency = "BTC"
		k.Label = "lightning"
		log.Printf("lightning_sent: SentAmount=%s", k.SentAmount)
	case "swap_in", "legacy_swap_in":
		// amount_msat is positive, and the mining_fee_sat is already accounted for.
		k.ReceivedAmount = fmt.Sprintf("%.8f", sats/100000000)
		k.ReceivedCurrency = "BTC"
		k.Label = "transfer"
		log.Printf("swap_in/legacy_swap_in: ReceivedAmount=%s", k.ReceivedAmount)
	case "swap_out":
		// amount_msat is negative, and the mining_fee_sat is already accounted for.
		k.SentAmount = fmt.Sprintf("%.8f", math.Abs(sats)/100000000)
		k.SentCurrency = "BTC"
		k.Label = "transfer"
		log.Printf("swap_out: SentAmount=%s", k.SentAmount)
	case "channel_open", "legacy_pay_to_open":
		// Channel open involves a received amount, and the service fee is already accounted for.
		k.ReceivedAmount = fmt.Sprintf("%.8f", sats/100000000)
		k.ReceivedCurrency = "BTC"
		k.Label = "deposit"
		log.Printf("channel_open/legacy_pay_to_open: ReceivedAmount=%s", k.ReceivedAmount)
	case "channel_close":
		// Koinly expects only the fee to be tracked, no balance change.
		k.SentAmount = ""
		k.SentCurrency = ""
		k.ReceivedAmount = ""
		k.ReceivedCurrency = ""
		k.FeeAmount = fmt.Sprintf("%.8f", math.Abs(sats)/100000000)
		k.FeeCurrency = "BTC"
		k.Label = "cost"
		log.Printf("channel_close: FeeAmount=%s", k.FeeAmount)
	default:
		log.Printf("Unknown transaction type for Koinly conversion: %s", p.Type)
	}

	return k
}

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




