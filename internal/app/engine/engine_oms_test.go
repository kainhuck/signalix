package engine

import (
	"github.com/kainhuck/signalix/internal/app/market"
	mktperp "github.com/kainhuck/signalix/internal/app/market/perp"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
)

func testOMSExecutors(ex ports.Exchange) (map[models.Market]market.MarketExecutor, *mktperp.PerpExecutor, error) {
	pe, err := mktperp.NewPerpExecutor(mktperp.PerpExecutorConfig{Exchange: ex})
	if err != nil {
		return nil, nil, err
	}
	return map[models.Market]market.MarketExecutor{
		models.MarketPerp: pe,
	}, pe, nil
}
