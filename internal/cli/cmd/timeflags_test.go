package cmd

import (
	"testing"
	"time"
)

func TestParseOptionalTimeFlag_empty(t *testing.T) {
	ms, err := parseOptionalTimeFlag("--since", "")
	if err != nil {
		t.Fatal(err)
	}
	if ms != 0 {
		t.Fatalf("ms = %d, want 0", ms)
	}
}

func TestParseOptionalTimeFlag_rfc3339Nano(t *testing.T) {
	want := time.Date(2026, 5, 26, 10, 0, 0, 123456789, time.UTC)
	ms, err := parseOptionalTimeFlag("--since", want.Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	if ms != want.UnixMilli() {
		t.Fatalf("ms = %d, want %d", ms, want.UnixMilli())
	}
}

func TestParseOptionalTimeFlag_invalid(t *testing.T) {
	_, err := parseOptionalTimeFlag("--since", "not-a-time")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseListTimeFlags(t *testing.T) {
	start, end, err := parseListTimeFlags("2026-05-26T00:00:00Z", "2026-05-26T12:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if start == 0 || end == 0 || start >= end {
		t.Fatalf("start=%d end=%d", start, end)
	}
}
