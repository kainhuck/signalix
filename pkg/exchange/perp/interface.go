package perp

import "context"

// Connector 生命周期与探活。
type Connector interface {
	Ping(ctx context.Context) error
	Close(ctx context.Context) error
}

// InstrumentSource 合约元数据。
type InstrumentSource interface {
	ListContractMeta(ctx context.Context) ([]*ContractMeta, error)
}

// MarketHistory 公共历史行情（REST K 线等）。
type MarketHistory interface {
	ListCandlesticks(ctx context.Context, q *ListCandlesticksQuery) ([]*CandlestickSnapshot, error)
}

// MarketSession 公共行情。
type MarketSession interface {
	Subscribe(ctx context.Context, subs []*Subscription) error
	Unsubscribe(ctx context.Context, subs []*Subscription) error
	PublicEvents() <-chan *PublicEvent
}

// Trading 交易。
type Trading interface {
	Place(ctx context.Context, req *PlaceRequest) (*OrderSnapshot, error)
	Cancel(ctx context.Context, p *CancelParams) error
	GetOrder(ctx context.Context, contract Contract, orderID string) (*OrderSnapshot, error)
}

// AccountView 账户快照。
type AccountView interface {
	Positions(ctx context.Context) ([]*PositionSnapshot, error)
	Position(ctx context.Context, contract Contract) (*PositionSnapshot, error)
	Balance(ctx context.Context) (*BalanceView, error)
}

// UserSession 私有推送。
type UserSession interface {
	UserEvents() <-chan *UserEvent
}

// Live 聚合常用能力。
type Live interface {
	Connector
	InstrumentSource
	MarketHistory
	MarketSession
	Trading
	AccountView
	UserSession
}
