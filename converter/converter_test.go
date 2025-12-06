package converter

import (
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestParsePhoenixRecord(t *testing.T) {
	record := []string{
		"2024-05-01T12:00:00.000Z", // timestamp
		"unused1",
		"lightning_received", // type
		"123456789",          // amount_msat
		"unused2",
		"unused3",
		"0", // mining fee sat
		"unused4",
		"0", // service fee msat
		"unused5",
		"unused6",
		"txid123", // transaction id
		"unused7",
		"test description", // description
	}
	p, err := ParsePhoenixRecord(record)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.Timestamp.Equal(time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("timestamp parsed incorrectly: %v", p.Timestamp)
	}
	if p.Type != "lightning_received" || p.AmountMillisats != 123456789 || p.MiningFeeSat != 0 || p.ServiceFeeMsat != 0 || p.TransactionID != "txid123" || p.Description != "test description" {
		t.Errorf("parsed struct mismatch: %+v", p)
	}
}

func TestToKoinlyRecordLightningReceived(t *testing.T) {
	p := &PhoenixRecord{
		Timestamp:       time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC),
		Type:            "lightning_received",
		AmountMillisats: 1000000000, // 1,000,000 sats
		TransactionID:   "tx1",
		Description:     "desc",
	}
	k, diff := ToKoinlyRecord(p)
	if math.Abs(diff) > 1e-9 {
		t.Errorf("expected zero rounding diff, got %f", diff)
	}
	if k.ReceivedAmount != "0.01000000" || k.ReceivedCurrency != "BTC" || k.Label != "lightning" {
		t.Errorf("unexpected koinly record: %+v", k)
	}
}

func TestToKoinlyRecordLightningSent(t *testing.T) {
	p := &PhoenixRecord{
		Timestamp:       time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC),
		Type:            "lightning_sent",
		AmountMillisats: -200000000, // -200,000 sats
		TransactionID:   "tx2",
		Description:     "desc",
	}
	k, diff := ToKoinlyRecord(p)
	if math.Abs(diff) > 1e-9 {
		t.Errorf("expected zero rounding diff, got %f", diff)
	}
	if k.SentAmount != "0.00200000" || k.SentCurrency != "BTC" || k.Label != "lightning" {
		t.Errorf("unexpected koinly record: %+v", k)
	}
}

func TestToKoinlyRecordChannelClose(t *testing.T) {
	p := &PhoenixRecord{
		Timestamp:       time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC),
		Type:            "channel_close",
		AmountMillisats: -150000, // -150 sats
		TransactionID:   "tx3",
		Description:     "desc",
	}
	k, diff := ToKoinlyRecord(p)
	if math.Abs(diff) > 1e-9 {
		t.Errorf("expected zero rounding diff, got %f", diff)
	}
	if k.FeeAmount != "0.00000150" || k.FeeCurrency != "BTC" || k.Label != "cost" {
		t.Errorf("unexpected koinly record: %+v", k)
	}
}

func TestFinalBalanceSampleCSV(t *testing.T) {
	// Need to fix path to testdata since we are in converter package
	f, err := os.Open(filepath.Join("..", "testdata", "sample_phoenix.csv"))
	if err != nil {
		t.Fatalf("failed to read csv: %v", err)
	}
	defer f.Close()

	records, err := ReadPhoenixCSV(f)
	if err != nil {
		t.Fatalf("failed to read csv records: %v", err)
	}
	var total float64
	for _, p := range records {
		k, diff := ToKoinlyRecord(p)
		if math.Abs(diff) > 1e-9 {
			t.Errorf("unexpected rounding diff %f", diff)
		}
		if k.ReceivedAmount != "" {
			v, err := strconv.ParseFloat(k.ReceivedAmount, 64)
			if err != nil {
				t.Fatalf("bad received amount: %v", err)
			}
			total += v
		}
		if k.SentAmount != "" {
			v, err := strconv.ParseFloat(k.SentAmount, 64)
			if err != nil {
				t.Fatalf("bad sent amount: %v", err)
			}
			total -= v
		}
		if k.FeeAmount != "" {
			v, err := strconv.ParseFloat(k.FeeAmount, 64)
			if err != nil {
				t.Fatalf("bad fee amount: %v", err)
			}
			total -= v
		}
	}
	expected := 0.00157
	if math.Abs(total-expected) > 1e-8 {
		t.Errorf("expected final balance %.8f BTC, got %.8f BTC", expected, total)
	}
}
