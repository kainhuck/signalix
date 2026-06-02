package gateio

import "testing"

func TestNormalizeGateOrderText_UUID(t *testing.T) {
	t.Parallel()

	long := "550e8400-e29b-41d4-a716-446655440000"
	got := normalizeGateOrderText(long)
	if len(got) > gateOrderTextMaxLen {
		t.Fatalf("len %d > %d: %q", len(got), gateOrderTextMaxLen, got)
	}
	if got[:2] != "t-" {
		t.Fatalf("want t- prefix, got %q", got)
	}
	if normalizeGateOrderText(long) != got {
		t.Fatal("expected stable hash")
	}
}

func TestNormalizeGateOrderText_short(t *testing.T) {
	t.Parallel()

	got := normalizeGateOrderText("abc123")
	if got != "t-abc123" {
		t.Fatalf("got %q", got)
	}
}

func TestClientTagFromLocal_alias(t *testing.T) {
	t.Parallel()

	c := &Client{}
	id := "ord-local"
	if c.TagFromLocal(id) != normalizeGateOrderText(id) {
		t.Fatal("TagFromLocal should match normalizeGateOrderText")
	}
}

func TestLocalIDFromGateText_short(t *testing.T) {
	t.Parallel()

	got := localIDFromGateText("t-abc123")
	if got != "abc123" {
		t.Fatalf("got %q", got)
	}
}

func TestLocalIDFromGateText_hashedUUID(t *testing.T) {
	t.Parallel()

	long := "550e8400-e29b-41d4-a716-446655440000"
	tag := normalizeGateOrderText(long)
	if got := localIDFromGateText(tag); got != "" {
		t.Fatalf("hashed gate text should not reverse, got %q", got)
	}
	c := &Client{}
	if local, ok := c.LocalFromTag(tag); ok || local != "" {
		t.Fatalf("LocalFromTag hashed: local=%q ok=%v", local, ok)
	}
}

func TestLocalIDFromGateText_empty(t *testing.T) {
	t.Parallel()

	if localIDFromGateText("") != "" {
		t.Fatal("empty text")
	}
	if localIDFromGateText("abc123") != "" {
		t.Fatal("missing t- prefix")
	}
}
