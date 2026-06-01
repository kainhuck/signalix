package market

import "github.com/kainhuck/signalix/internal/models"

// MarketUpdateKind 行情更新类型。
type MarketUpdateKind int

const (
	MarketUpdateTicker MarketUpdateKind = iota
	MarketUpdateKline
)

// MarketUpdate 分发给引擎的行情更新（市场中性：携带 models 层 IPC 载荷）。
type MarketUpdate struct {
	StrategyName string
	Kind         MarketUpdateKind
	Ticker       *models.Ticker
	Kline        *models.Kline
}
