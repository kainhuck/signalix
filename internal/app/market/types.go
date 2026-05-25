package market

import "github.com/kainhuck/signalix/pkg/exchange/perp"

// MarketUpdateKind 行情更新类型。
type MarketUpdateKind int

const (
	MarketUpdateTicker MarketUpdateKind = iota
	MarketUpdateKline
)

// MarketUpdate 分发给引擎的行情更新。
type MarketUpdate struct {
	StrategyName string
	Kind         MarketUpdateKind
	Ticker       *perp.TickerSnapshot
	Kline        *perp.CandlestickSnapshot
}
