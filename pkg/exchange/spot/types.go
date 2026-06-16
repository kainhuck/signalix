package spot

import "time"

// Side 订单方向。
type Side string

const (
	SideBuy  Side = "Buy"
	SideSell Side = "Sell"
)

// OrderType 当前适配器保证支持的类型。
type OrderType string

const (
	OrderTypeMarket OrderType = "Market"
	OrderTypeLimit  OrderType = "Limit"
)

// TimeInForce 限价单时效。
type TimeInForce string

const (
	TIFGTC TimeInForce = "GTC"
	TIFIOC TimeInForce = "IOC"
	TIFFOK TimeInForce = "FOK"
	TIFPOC TimeInForce = "POC"
)

// OrderStatus 归一化订单状态。
type OrderStatus string

const (
	OrderPending       OrderStatus = "Pending"
	OrderSubmitted     OrderStatus = "Submitted"
	OrderPartialFilled OrderStatus = "PartialFilled"
	OrderFilled        OrderStatus = "Filled"
	OrderCancelled     OrderStatus = "Cancelled"
	OrderRejected      OrderStatus = "Rejected"
)

// PlaceRequest 下单参数；数量/价格使用 string，与交易所 API 对齐。
type PlaceRequest struct {
	Pair        Pair
	Side        Side
	Type        OrderType
	Size        string  // base 数量（Limit；Market Sell）
	QuoteAmount *string // Market Buy 时必填：quote 数量
	Price       *string
	TimeInForce TimeInForce
	ClientID    string
	Account     string // 默认 spot
}

// CancelParams 撤单。
type CancelParams struct {
	Pair          Pair
	OrderID       string
	ClientOrderID string
}

// BalanceView 单币种余额。
type BalanceView struct {
	Currency  string
	Total     string
	Available string
	Frozen    string
	UpdatedAt time.Time
}

// PairMeta 现货交易对规则。
type PairMeta struct {
	Pair            Pair
	MinBaseAmount   string
	MinQuoteAmount  string
	MaxBaseAmount   string
	AmountPrecision int32
	PricePrecision  int32
	TradeStatus     string
}

// CandlestickSnapshot K 线快照。
type CandlestickSnapshot struct {
	Pair         Pair
	Interval     string
	Open         string
	High         string
	Low          string
	Close        string
	Volume       string
	VolumeBase   string
	TimestampSec int64
	WindowClosed bool
}

// TickerSnapshot 现货 ticker（WebSocket 等）。
type TickerSnapshot struct {
	Pair            Pair
	Last            string
	HighestBid      string
	LowestAsk       string
	ChangePct24h    string
	Volume24hBase   string
	Volume24hQuote  string
	High24h         string
	Low24h          string
	TimestampMillis int64
}

// BalanceUpdateSnapshot spot.balances 推送。
type BalanceUpdateSnapshot struct {
	Currency   string
	Total      string
	Available  string
	Frozen     string
	Change     string
	ChangeType string
	UpdatedAt  time.Time
}

// ListCandlesticksQuery 查询历史 K 线（Gate GET /spot/candlesticks）。
type ListCandlesticksQuery struct {
	Pair     Pair
	Interval string
	Limit    int
	From     int64
	To       int64
}

// OrderSnapshot 订单快照。
type OrderSnapshot struct {
	OrderID         string
	ExchangeOrderID string
	ClientID        string
	Pair            Pair
	Side            Side
	Type            OrderType
	Size            string
	Price           string
	FilledSize      string
	AvgPrice        string
	Status          OrderStatus
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ConnectParts 控制建立哪些链路。
type ConnectParts struct {
	REST      bool
	PublicWS  bool
	PrivateWS bool
}

// DefaultConnectREST 默认仅 REST（MF-1）。
func DefaultConnectREST() ConnectParts {
	return ConnectParts{REST: true}
}

// Subscription WebSocket 订阅描述（MF-2 实现）。
type Subscription struct {
	Channel string
	Pairs   []Pair
	Payload []string
}

// --- user events (MF-2) ---

const (
	UserOrderUpdate   UserKind = "OrderUpdate"
	UserBalanceUpdate UserKind = "BalanceUpdate"
	UserTradeUpdate   UserKind = "TradeUpdate"
)

type (
	UserKind string

	UserEvent struct {
		Kind    UserKind
		Payload UserPayload
	}
)

// --- public events ---

const (
	PublicTicker      PublicKind = "Ticker"
	PublicCandlestick PublicKind = "Candlestick"
)

type (
	PublicKind string

	PublicEvent struct {
		Kind    PublicKind
		Payload PublicPayload
	}
)
