package perp

import "testing"

func TestNormalizeClientOrderID_UUID(t *testing.T) {
	t.Parallel()

	long := "550e8400-e29b-41d4-a716-446655440000"
	got := NormalizeClientOrderID(long)
	if len(got) > GateOrderTextMaxLen {
		t.Fatalf("len %d > %d: %q", len(got), GateOrderTextMaxLen, got)
	}
	if got[:2] != "t-" {
		t.Fatalf("want t- prefix, got %q", got)
	}
	if NormalizeClientOrderID(long) != got {
		t.Fatal("expected stable hash")
	}
}

func TestNormalizeClientOrderID_short(t *testing.T) {
	t.Parallel()

	got := NormalizeClientOrderID("abc123")
	if got != "t-abc123" {
		t.Fatalf("got %q", got)
	}
}

func TestGateTextFromClientOrderID_alias(t *testing.T) {
	t.Parallel()

	id := "ord-local"
	if GateTextFromClientOrderID(id) != NormalizeClientOrderID(id) {
		t.Fatal("GateTextFromClientOrderID should match NormalizeClientOrderID")
	}
}

func TestLocalClientIDFromGateText_short(t *testing.T) {
	t.Parallel()

	got := LocalClientIDFromGateText("t-abc123")
	if got != "abc123" {
		t.Fatalf("got %q", got)
	}
}

func TestLocalClientIDFromGateText_hashedUUID(t *testing.T) {
	t.Parallel()

	long := "550e8400-e29b-41d4-a716-446655440000"
	gateText := NormalizeClientOrderID(long)
	if got := LocalClientIDFromGateText(gateText); got != "" {
		t.Fatalf("hashed gate text should not reverse, got %q", got)
	}
}

func TestLocalClientIDFromGateText_empty(t *testing.T) {
	t.Parallel()

	if LocalClientIDFromGateText("") != "" {
		t.Fatal("empty text")
	}
	if LocalClientIDFromGateText("abc123") != "" {
		t.Fatal("missing t- prefix")
	}
}
