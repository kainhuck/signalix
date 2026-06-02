package engine

import (
	"github.com/kainhuck/signalix/internal/app/market"
	mktperp "github.com/kainhuck/signalix/internal/app/market/perp"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
)

func testOMSExecutors(ex ports.Exchange) (map[models.Market]market.MarketExecutor, *mktperp.PerpExecutor) {
	pe := mktperp.NewPerpExecutor(mktperp.PerpExecutorConfig{Exchange: ex})
	return map[models.Market]market.MarketExecutor{
		models.MarketPerp: pe,
	}, pe
}
