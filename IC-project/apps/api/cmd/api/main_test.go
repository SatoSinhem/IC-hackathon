package main

import (
	"testing"
	"time"
)

func TestAlmatyTimezoneIsAvailable(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Almaty")
	if err != nil {
		t.Fatalf("Asia/Almaty is not available: %v", err)
	}
	// Kazakhstan has used a single UTC+5 zone since March 2024.
	_, offset := time.Date(2026, time.October, 1, 12, 0, 0, 0, loc).Zone()
	if offset != 5*60*60 {
		t.Fatalf("offset = %d seconds, want %d", offset, 5*60*60)
	}
}
