package perp

import "time"

// Contract 永续合约标识，规范写法为 BASE/QUOTE，例如 BTC/USDT。
// 各交易所适配器在 REST/WebSocket 边界负责与原生符号互转。
type Contract string

// Side 订单方向。
type Side string

const (
	SideBuy  Side = "Buy"
	SideSell Side = "Sell"
)

// PositionSide 持仓方向。
type PositionSide string

const (
	PositionLong  PositionSide = "Long"
	PositionShort PositionSide = "Short"
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
	Contract    Contract
	Side        Side
	Type        OrderType
	Size        string
	Price       *string
	TimeInForce TimeInForce
	ReduceOnly  bool
	ClientID    string
}

// CancelParams 撤单：OrderID 与 ClientOrderID 二选一（Gate 均走 order_id 路径参数）。
type CancelParams struct {
	Contract      Contract
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

// ContractMeta 合约规则。
type ContractMeta struct {
	Contract          Contract
	QuantoMultiplier  string // 每张合约标的乘数（Gate quanto_multiplier）
	OrderSizeMin      string
	OrderSizeMax      string
	OrderPriceStep    string
	OrderSizeStep     string
	EnableDecimalSize bool // Gate enable_decimal：为 false 时 size 须整数张数
}

// ListCandlesticksQuery 查询历史 K 线（Gate GET /futures/{settle}/candlesticks）。
// Limit 与 From/To 互斥：指定 From 或 To 时勿设 Limit（由适配器校验）。
// 单次最多 2000 根（Gate 文档上限）。
type ListCandlesticksQuery struct {
	Contract Contract
	Interval string
	Limit    int
	From     int64  // Unix 秒；0 表示不传
	To       int64  // Unix 秒；0 表示不传
	Timezone string // utc0 / utc8 / all；空则交易所默认
}

// Subscription WebSocket 订阅描述。
type Subscription struct {
	Channel   string
	Contracts []Contract
	// Payload 频道附加参数，由交易所适配器解释。例如：
	//   futures.candlesticks — Payload[0] 为周期（1m、5m 等），Contracts 为合约列表；
	//   futures.order_book — Payload[0] 档位数、Payload[1] 推送间隔；
	//   futures.balances — 无需 Contracts/Payload，仅需 PrivateWS + userID；
	//   futures.usertrades — Contracts 为空则订阅 !all，否则按合约列表订阅；
	//   futures.positions — 同上。
	Payload []string
}

// ConnectParts 控制建立哪些链路。
type ConnectParts struct {
	REST      bool
	PublicWS  bool
	PrivateWS bool
}

// DefaultConnectAll 默认：REST + 公网 WS（不含私有流）。
func DefaultConnectAll() ConnectParts {
	return ConnectParts{REST: true, PublicWS: true, PrivateWS: false}
}

// --- user events ---

const (
	UserOrderUpdate    UserKind = "OrderUpdate"
	UserBalanceUpdate  UserKind = "BalanceUpdate"
	UserTradeUpdate    UserKind = "TradeUpdate"
	UserPositionUpdate UserKind = "PositionUpdate"
)

type (
	// UserKind 用户私有事件。
	UserKind string

	// UserEvent 用户私有推送。
	UserEvent struct {
		Kind    UserKind
		Payload UserPayload
	}

	// BalanceUpdateSnapshot 余额变更推送（Gate futures.balances）。
	BalanceUpdateSnapshot struct {
		Currency   string
		Balance    string
		Change     string
		ChangeType string
		Text       string
		UserID     string
		UpdatedAt  time.Time
	}

	// TradeSnapshot 用户私有成交（Gate futures.usertrades）。
	TradeSnapshot struct {
		TradeID         string
		ExchangeOrderID string
		Contract        Contract
		Side            Side
		Size            string
		Price           string
		Role            string
		Fee             string
		PointFee        string
		Text            string
		CreatedAt       time.Time
	}

	// OrderSnapshot 订单快照。
	OrderSnapshot struct {
		OrderID         string
		ExchangeOrderID string
		Contract        Contract
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

	// PositionSnapshot 持仓快照。
	PositionSnapshot struct {
		Contract      Contract
		Side          PositionSide
		Size          string
		EntryPrice    string
		MarkPrice     string
		UnrealizedPnl string
		Leverage      int
		UpdatedAt     time.Time
	}
)

// --- public events ---

const (
	PublicTicker      PublicKind = "Ticker"
	PublicCandlestick PublicKind = "Candlestick"
)

type (
	// PublicKind 公共行情事件类型。
	PublicKind string

	// PublicEvent 公共推送。
	PublicEvent struct {
		Kind    PublicKind
		Payload PublicPayload
	}

	// TickerSnapshot ticker 聚合。
	TickerSnapshot struct {
		Contract        Contract
		Last            string
		MarkPrice       string
		IndexPrice      string
		FundingRate     string
		ChangePct24h    string
		Volume24h       string
		Volume24hBase   string
		Volume24hQuote  string
		OpenInterest    string
		Low24h          string
		High24h         string
		TimestampMillis int64
	}

	// CandlestickSnapshot K 线快照（字段语义与 Gate futures.candlesticks 推送对齐）。
	CandlestickSnapshot struct {
		Contract     Contract
		Interval     string // 10s, 1m, 5m, 15m, 30m, 1h, 4h, 8h, 1d, 7d
		Open         string
		High         string
		Low          string
		Close        string
		Volume       string
		VolumeBase   string
		TimestampSec int64
		WindowClosed bool
	}
)
