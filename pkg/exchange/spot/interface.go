package spot

import "context"

// Connector 生命周期与探活。
type Connector interface {
	Ping(ctx context.Context) error
	Close(ctx context.Context) error
}

// InstrumentSource 交易对元数据。
type InstrumentSource interface {
	ListPairMeta(ctx context.Context) ([]*PairMeta, error)
}

// MarketHistory 公共历史行情（REST K 线等）。
type MarketHistory interface {
	ListCandlesticks(ctx context.Context, q *ListCandlesticksQuery) ([]*CandlestickSnapshot, error)
}

// Trading 交易。
type Trading interface {
	Place(ctx context.Context, req *PlaceRequest) (*OrderSnapshot, error)
	Cancel(ctx context.Context, p *CancelParams) error
	GetOrder(ctx context.Context, pair Pair, orderID string) (*OrderSnapshot, error)
}

// AccountView 账户快照。
type AccountView interface {
	Balances(ctx context.Context) ([]*BalanceView, error)
	Balance(ctx context.Context, currency string) (*BalanceView, error)
}

// MarketSession 公共行情（MF-2）。
type MarketSession interface {
	Subscribe(ctx context.Context, subs []*Subscription) error
	Unsubscribe(ctx context.Context, subs []*Subscription) error
	PublicEvents() <-chan *PublicEvent
}

// UserSession 私有推送（MF-2）。
type UserSession interface {
	UserEvents() <-chan *UserEvent
}

// REST MF-1 交付范围。
type REST interface {
	Connector
	InstrumentSource
	MarketHistory
	Trading
	AccountView
}

// Live 聚合常用能力（MF-2 完整实现）。
type Live interface {
	REST
	MarketSession
	UserSession
}
