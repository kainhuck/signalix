package perp

import (
	"context"
	"fmt"

	"github.com/kainhuck/signalix/internal/app/decision"
	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/models"
)

// perpDecider 将既有 DecisionEngine 暴露为 market.MarketDecider（本专项 perp 唯一决策实现）。
type perpDecider struct {
	eng *decision.DecisionEngine
}

// NewPerpDecider 构造 perp 的 market.MarketDecider。
func NewPerpDecider(eng *decision.DecisionEngine) market.MarketDecider {
	return &perpDecider{eng: eng}
}

// Decide 满足 market.MarketDecider。
func (p *perpDecider) Decide(ctx context.Context, strategy string, sig *models.Signal) (*models.Order, error) {
	if p == nil || p.eng == nil {
		return nil, fmt.Errorf("perp decider not configured")
	}
	return p.eng.Decide(ctx, strategy, sig)
}

var _ market.MarketDecider = (*perpDecider)(nil)
