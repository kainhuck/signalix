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

func TestEnabledMarkets_rejectsSpot(t *testing.T) {
	_, err := (&Config{Markets: MarketsConfig{Enabled: []string{"spot"}}}).EnabledMarkets()
	if err == nil {
		t.Fatal("expected error for spot")
	}
}
