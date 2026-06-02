package perp

import (
	"context"
	"testing"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/testutil"
)

func TestPerpMarket_implementsMarket(t *testing.T) {
	var _ market.Market = (*PerpMarket)(nil)
}

func TestPerpFromMarkets(t *testing.T) {
	ex := testutil.NewStubExchange()
	pm, err := NewPerpMarket(context.Background(), PerpMarketConfig{Exchange: ex})
	if err != nil {
		t.Fatal(err)
	}
	got, err := PerpFromMarkets(map[models.Market]market.Market{models.MarketPerp: pm})
	if err != nil || got != pm {
		t.Fatalf("PerpFromMarkets: %v err=%v", got, err)
	}
}
