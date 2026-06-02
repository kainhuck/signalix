package models

import (
	"encoding/json"
	"testing"
)

func TestParseMarket(t *testing.T) {
	cases := []struct {
		in      string
		want    Market
		wantErr bool
	}{
		{"", MarketPerp, false},
		{"perp", MarketPerp, false},
		{"PERP", MarketPerp, false},
		{" spot ", MarketSpot, false},
		{"spot", MarketSpot, false},
		{"options", "", true},
	}
	for _, c := range cases {
		got, err := ParseMarket(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseMarket(%q): expected error, got %q", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseMarket(%q): unexpected error %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseMarket(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMarketValid(t *testing.T) {
	if !MarketPerp.Valid() || !MarketSpot.Valid() {
		t.Fatal("perp and spot must be valid")
	}
	if Market("futures").Valid() {
		t.Fatal("unknown market must be invalid")
	}
}

func TestViewsJSONStable(t *testing.T) {
	pv := PositionView{Market: MarketPerp, Symbol: "BTC_USDT", Side: "long", Size: "3"}
	b, err := json.Marshal(pv)
	if err != nil {
		t.Fatalf("marshal PositionView: %v", err)
	}
	var round PositionView
	if err := json.Unmarshal(b, &round); err != nil {
		t.Fatalf("unmarshal PositionView: %v", err)
	}
	if round.Market != MarketPerp || round.Symbol != "BTC_USDT" || round.Size != "3" {
		t.Fatalf("PositionView round-trip mismatch: %+v", round)
	}

	oe := OrderEvent{Market: MarketPerp, ClientID: "id-1", Status: OrderStatusFilled, FilledSize: "1"}
	if _, err := json.Marshal(oe); err != nil {
		t.Fatalf("marshal OrderEvent: %v", err)
	}

	bv := BalanceView{Currency: "USDT", Total: "100", Available: "90"}
	if _, err := json.Marshal(bv); err != nil {
		t.Fatalf("marshal BalanceView: %v", err)
	}
}
