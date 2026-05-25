package gateio

import "testing"

func TestNormalizeClientOrderID_UUID(t *testing.T) {
	t.Parallel()

	long := "550e8400-e29b-41d4-a716-446655440000"
	got := normalizeClientOrderID(long)
	if len(got) > gateOrderTextMaxLen {
		t.Fatalf("len %d > %d: %q", len(got), gateOrderTextMaxLen, got)
	}
	if got[:2] != "t-" {
		t.Fatalf("want t- prefix, got %q", got)
	}
	// 稳定映射
	if normalizeClientOrderID(long) != got {
		t.Fatal("expected stable hash")
	}
}

func TestNormalizeClientOrderID_short(t *testing.T) {
	t.Parallel()

	got := normalizeClientOrderID("abc123")
	if got != "t-abc123" {
		t.Fatalf("got %q", got)
	}
}
