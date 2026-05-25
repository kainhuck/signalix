package risk

import (
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"github.com/shopspring/decimal"
)

// OpensExposure 信号是否可能增加账户风险敞口（开多/开空）。
func OpensExposure(sig *models.Signal) bool {
	if sig == nil {
		return true
	}
	switch sig.Direction {
	case models.DirectionLong, models.DirectionShort:
		return true
	default:
		return false
	}
}

// IsIncreasingExposure 判定本笔订单是否为同向增仓。
func IsIncreasingExposure(sig *models.Signal, position *perp.PositionSnapshot) bool {
	if sig == nil {
		return false
	}
	switch sig.Direction {
	case models.DirectionLong:
		if position == nil {
			return true
		}
		size, _ := decimal.NewFromString(position.Size)
		if !size.IsPositive() {
			return true
		}
		return position.Side == perp.PositionLong
	case models.DirectionShort:
		if position == nil {
			return true
		}
		size, _ := decimal.NewFromString(position.Size)
		if !size.IsPositive() {
			return true
		}
		return position.Side == perp.PositionShort
	default:
		return false
	}
}
