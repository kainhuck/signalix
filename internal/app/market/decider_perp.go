package market

import (
	"context"
	"fmt"

	"github.com/kainhuck/signalix/internal/app/decision"
	"github.com/kainhuck/signalix/internal/models"
)

// perpDecider 将既有 DecisionEngine 暴露为 MarketDecider（本专项 perp 唯一决策实现）。
type perpDecider struct {
	eng *decision.DecisionEngine
}

// NewPerpDecider 构造 perp 的 MarketDecider。
func NewPerpDecider(eng *decision.DecisionEngine) MarketDecider {
	return &perpDecider{eng: eng}
}

// Decide 满足 MarketDecider。
func (p *perpDecider) Decide(ctx context.Context, strategy string, sig *models.Signal) (*models.Order, error) {
	if p == nil || p.eng == nil {
		return nil, fmt.Errorf("perp decider not configured")
	}
	return p.eng.Decide(ctx, strategy, sig)
}

var _ MarketDecider = (*perpDecider)(nil)
