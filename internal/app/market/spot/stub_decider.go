package spot

import (
	"context"

	"github.com/kainhuck/signalix/internal/models"
)

type safeDecider struct{}

func (safeDecider) Decide(ctx context.Context, strategy string, sig *models.Signal) (*models.Order, error) {
	_ = ctx
	_ = strategy
	_ = sig
	return nil, nil
}
