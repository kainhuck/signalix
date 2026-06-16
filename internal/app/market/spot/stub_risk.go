package spot

import (
	"context"
	"fmt"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
)

type safeRisk struct{}

func (safeRisk) BuildRiskContext(ctx context.Context, strategy string, sig *models.Signal, o *models.Order) (*ports.RiskContext, error) {
	_ = ctx
	_ = strategy
	_ = sig
	_ = o
	return nil, fmt.Errorf("spot risk not implemented")
}
