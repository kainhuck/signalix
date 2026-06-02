package perp

import (
	"time"

	"github.com/kainhuck/signalix/internal/app/projection"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

// WrapProjectionRefreshHook 将 models 层持久化钩子适配为 projection.RefreshHook。
func WrapProjectionRefreshHook(fn func(balance *models.BalanceView, positions []*models.PositionView, revision uint64, at time.Time)) projection.RefreshHook {
	if fn == nil {
		return nil
	}
	return func(balance *perp.BalanceView, positions []*perp.PositionSnapshot, revision uint64, at time.Time) {
		var posViews []*models.PositionView
		if len(positions) > 0 {
			posViews = make([]*models.PositionView, 0, len(positions))
			for _, pv := range positions {
				if pv == nil {
					continue
				}
				posViews = append(posViews, positionViewFromPerp(pv))
			}
		}
		fn(balanceViewFromPerp(balance), posViews, revision, at)
	}
}
