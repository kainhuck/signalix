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
		Currency   string    `json:"currency"`
		Balance    string    `json:"balance"`
		Change     string    `json:"change"`
		ChangeType string    `json:"change_type"`
		Text       string    `json:"text"`
		UserID     string    `json:"user_id"`
		UpdatedAt  time.Time `json:"updated_at"`
	}

	// TradeSnapshot 用户私有成交（Gate futures.usertrades）。
	TradeSnapshot struct {
		TradeID         string    `json:"trade_id"`
		ExchangeOrderID string    `json:"exchange_order_id"`
		Contract        Contract  `json:"contract"`
		Side            Side      `json:"side"`
		Size            string    `json:"size"`
		Price           string    `json:"price"`
		Role            string    `json:"role"`
		Fee             string    `json:"fee"`
		PointFee        string    `json:"point_fee"`
		Text            string    `json:"text"`
		CreatedAt       time.Time `json:"created_at"`
	}

	// OrderSnapshot 订单快照。
	OrderSnapshot struct {
		OrderID         string      `json:"order_id"`
		ExchangeOrderID string      `json:"exchange_order_id"`
		Contract        Contract    `json:"contract"`
		Side            Side        `json:"side"`
		Type            OrderType   `json:"type"`
		Size            string      `json:"size"`
		Price           string      `json:"price"`
		FilledSize      string      `json:"filled_size"`
		AvgPrice        string      `json:"avg_price"`
		Status          OrderStatus `json:"status"`
		CreatedAt       time.Time   `json:"created_at"`
		UpdatedAt       time.Time   `json:"updated_at"`
	}

	// PositionSnapshot 持仓快照。
	PositionSnapshot struct {
		Contract      Contract     `json:"contract"`
		Side          PositionSide `json:"side"`
		Size          string       `json:"size"`
		EntryPrice    string       `json:"entry_price"`
		MarkPrice     string       `json:"mark_price"`
		UnrealizedPnl string       `json:"unrealized_pnl"`
		Leverage      int          `json:"leverage"`
		UpdatedAt     time.Time    `json:"updated_at"`
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
		Contract        Contract `json:"contract"`
		Last            string   `json:"last"`
		MarkPrice       string   `json:"mark_price"`
		IndexPrice      string   `json:"index_price"`
		FundingRate     string   `json:"funding_rate"`
		ChangePct24h    string   `json:"change_pct_24h"`
		Volume24h       string   `json:"volume_24h"`
		Volume24hBase   string   `json:"volume_24h_base"`
		Volume24hQuote  string   `json:"volume_24h_quote"`
		OpenInterest    string   `json:"open_interest"`
		Low24h          string   `json:"low_24h"`
		High24h         string   `json:"high_24h"`
		TimestampMillis int64    `json:"timestamp_millis"`
	}

	// CandlestickSnapshot K 线快照（与 Gate futures.candlesticks 推送字段对齐）。
	CandlestickSnapshot struct {
		Contract     Contract `json:"contract"`
		Interval     string   `json:"interval"` // 10s, 1m, 5m, 15m, 30m, 1h, 4h, 8h, 1d, 7d
		Open         string   `json:"open"`
		High         string   `json:"high"`
		Low          string   `json:"low"`
		Close        string   `json:"close"`
		Volume       string   `json:"volume"`
		VolumeBase   string   `json:"volume_base"`
		TimestampSec int64    `json:"timestamp_sec"`
		WindowClosed bool     `json:"window_closed"`
	}
)
