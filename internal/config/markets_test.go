package config

import (
	"testing"

	"github.com/kainhuck/signalix/internal/models"
)

func TestEnabledMarkets_default(t *testing.T) {
	m, err := (&Config{}).EnabledMarkets()
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 || m[0] != models.MarketPerp {
		t.Fatalf("got %v", m)
	}
}

func TestEnabledMarkets_allowsSpot(t *testing.T) {
	m, err := (&Config{Markets: MarketsConfig{Enabled: []string{"spot"}}}).EnabledMarkets()
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 || m[0] != models.MarketSpot {
		t.Fatalf("got %v", m)
	}
}

func TestEnabledMarkets_dedupesMixedMarkets(t *testing.T) {
	m, err := (&Config{Markets: MarketsConfig{Enabled: []string{"perp", "spot", "spot"}}}).EnabledMarkets()
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 2 || m[0] != models.MarketPerp || m[1] != models.MarketSpot {
		t.Fatalf("got %v", m)
	}
}
