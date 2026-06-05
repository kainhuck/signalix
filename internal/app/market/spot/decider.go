package spot

import (
	"context"
	"fmt"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/models"
)

type spotDecider struct{}

func newSpotDecider() market.MarketDecider {
	return &spotDecider{}
}

func (d *spotDecider) Decide(ctx context.Context, strategy string, sig *models.Signal) (*models.Order, error) {
	return nil, fmt.Errorf("spot decider not implemented")
}

var _ market.MarketDecider = (*spotDecider)(nil)
