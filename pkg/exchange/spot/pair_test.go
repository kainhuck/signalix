package spot

import "testing"

func TestCanonicalPair(t *testing.T) {
	tests := []struct {
		in   string
		want Pair
	}{
		{"BTC_USDT", "BTC/USDT"},
		{"btc/usdt", "BTC/USDT"},
		{"BTCUSDT", "BTC/USDT"},
		{"", ""},
	}
	for _, tc := range tests {
		if got := CanonicalPair(tc.in); got != tc.want {
			t.Fatalf("CanonicalPair(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
