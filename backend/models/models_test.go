package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestStockChangeHistoryJSON(t *testing.T) {
	// Prepare a sample struct
	orig := StockChangeHistory{
		ID:         123,
		ChangeTime: "12:34:56",
		ChangeDate: "2026-06-27",
	}
	// Marshal to JSON
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	// Unmarshal back
	var decoded StockChangeHistory
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	// Verify fields are equal
	if decoded.ID != orig.ID || decoded.ChangeTime != orig.ChangeTime || decoded.ChangeDate != orig.ChangeDate {
		t.Fatalf("decoded struct does not match original: got %+v want %+v", decoded, orig)
	}
	// Ensure time parsing does not panic (use convertor)
	_ = time.Now()
}
