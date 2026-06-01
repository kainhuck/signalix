package models

import "time"

// PositionView 市场中性持仓视图。
//
// perp：Side = "long"/"short"，Size = 张数，含 EntryPrice/MarkPrice/Leverage。
// spot：Size = base 余额 total（持有的 base 数量），Side = "long"（持有）或空；
// EntryPrice/MarkPrice/UnrealizedPnl/Leverage 留空。可用/冻结明细走 BalanceView，
// 本结构不携带 Available/Frozen（避免 perp 出现无意义字段）。
type PositionView struct {
	Market        Market    `json:"market"`
	Symbol        string    `json:"symbol"`
	Side          string    `json:"side,omitempty"`
	Size          string    `json:"size"`
	EntryPrice    string    `json:"entry_price,omitempty"`
	MarkPrice     string    `json:"mark_price,omitempty"`
	UnrealizedPnl string    `json:"unrealized_pnl,omitempty"`
	Leverage      string    `json:"leverage,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// BalanceView 市场中性余额视图（perp settle 币 / spot 任一币种）。
type BalanceView struct {
	Currency  string    `json:"currency"`
	Total     string    `json:"total"`
	Available string    `json:"available"`
	Frozen    string    `json:"frozen,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// OrderEvent 归一化订单事件：来自用户流或主动 Sync。
//
// ClientID = 本地 Order.ID（下单时作为所侧幂等键传入），用于可靠匹配本地订单，
// 避免多市场下 ExchangeID 碰撞。
type OrderEvent struct {
	Market     Market      `json:"market"`
	ExchangeID string      `json:"exchange_id,omitempty"`
	ClientID   string      `json:"client_id,omitempty"`
	Status     OrderStatus `json:"status"`
	FilledSize string      `json:"filled_size"`
	UpdatedAt  time.Time   `json:"updated_at"`
}
