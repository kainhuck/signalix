package engine

import (
	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
)

func testOMSExecutors(ex ports.Exchange) (map[models.Market]market.MarketExecutor, *market.PerpExecutor) {
	pe := market.NewPerpExecutor(market.PerpExecutorConfig{Exchange: ex})
	return map[models.Market]market.MarketExecutor{
		models.MarketPerp: pe,
	}, pe
}
