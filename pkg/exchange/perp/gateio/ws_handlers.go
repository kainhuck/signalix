package gateio

import (
	"math"
	"strings"
	"time"

	"github.com/kainhuck/signalix/pkg/exchange/perp"

	"github.com/gate/gateapi-go/v7"
)

func (h *wsHub) handleTicker(msg map[string]interface{}) {
	list, ok := msg["result"].([]interface{})
	if !ok || len(list) == 0 {
		return
	}
	row, ok := list[0].(map[string]interface{})
	if !ok {
		return
	}
	contract, _ := row["contract"].(string)
	cc := perp.CanonicalContract(contract)
	snap := &perp.TickerSnapshot{
		Contract:        cc,
		Last:            str(row["last"]),
		MarkPrice:       str(row["mark_price"]),
		IndexPrice:      str(row["index_price"]),
		FundingRate:     str(row["funding_rate"]),
		ChangePct24h:    strPct(str(row["change_percentage"])),
		Volume24h:       str(row["volume_24h"]),
		Volume24hBase:   str(row["volume_24h_base"]),
		Volume24hQuote:  str(row["volume_24h_quote"]),
		OpenInterest:    str(row["total_size"]),
		Low24h:          str(row["low_24h"]),
		High24h:         str(row["high_24h"]),
		TimestampMillis: time.Now().UnixMilli(),
	}
	ev, err := perp.NewPublicEvent(perp.PublicTicker, snap)
	if err != nil {
		h.c.log.ErrorContext(h.hubCtx, "ws public event", "err", err)
		return
	}
	select {
	case h.c.pubCh <- ev:
	default:
	}
}

func strPct(change string) string {
	f := parseFloat(change)
	if f == 0 {
		return "0"
	}
	return trimFloatString(f / 100)
}

func str(v interface{}) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return trimFloatString(t)
	default:
		return ""
	}
}

func int64Val(v interface{}) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	default:
		return 0
	}
}

func boolVal(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	default:
		return false
	}
}

// parseCandlestickName 解析 Gate 字段 n，形如 "1m_BTC_USDT"。
func parseCandlestickName(n string) (interval string, contract perp.Contract) {
	n = strings.TrimSpace(n)
	i := strings.Index(n, "_")
	if i <= 0 || i >= len(n)-1 {
		return "", ""
	}
	return n[:i], perp.CanonicalContract(n[i+1:])
}

func candlestickFromWSRow(row map[string]interface{}) (*perp.CandlestickSnapshot, bool) {
	interval, contract := parseCandlestickName(str(row["n"]))
	if contract == "" {
		return nil, false
	}
	return &perp.CandlestickSnapshot{
		Contract:     contract,
		Interval:     interval,
		Open:         str(row["o"]),
		High:         str(row["h"]),
		Low:          str(row["l"]),
		Close:        str(row["c"]),
		Volume:       str(row["v"]),
		VolumeBase:   str(row["a"]),
		TimestampSec: int64Val(row["t"]),
		WindowClosed: boolVal(row["w"]),
	}, true
}

func (h *wsHub) handleCandlestick(msg map[string]interface{}) {
	raw, ok := msg["result"]
	if !ok {
		return
	}
	var rows []map[string]interface{}
	switch v := raw.(type) {
	case []interface{}:
		for _, e := range v {
			row, ok := e.(map[string]interface{})
			if !ok {
				continue
			}
			rows = append(rows, row)
		}
	case map[string]interface{}:
		rows = append(rows, v)
	default:
		return
	}
	for _, row := range rows {
		snap, ok := candlestickFromWSRow(row)
		if !ok {
			continue
		}
		ev, err := perp.NewPublicEvent(perp.PublicCandlestick, snap)
		if err != nil {
			h.c.log.ErrorContext(h.hubCtx, "ws public event", "err", err)
			continue
		}
		select {
		case h.c.pubCh <- ev:
		default:
		}
	}
}

func balanceUpdateFromWSRow(row map[string]interface{}) (*perp.BalanceUpdateSnapshot, bool) {
	currency := strings.ToUpper(str(row["currency"]))
	if currency == "" {
		return nil, false
	}
	ts := int64Val(row["time_ms"])
	if ts == 0 {
		ts = int64Val(row["time"]) * 1000
	}
	var updatedAt time.Time
	if ts > 0 {
		updatedAt = time.UnixMilli(ts)
	} else {
		updatedAt = time.Now()
	}
	return &perp.BalanceUpdateSnapshot{
		Currency:   currency,
		Balance:    str(row["balance"]),
		Change:     str(row["change"]),
		ChangeType: str(row["type"]),
		Text:       str(row["text"]),
		UserID:     str(row["user"]),
		UpdatedAt:  updatedAt,
	}, true
}

func (h *wsHub) handleBalances(msg map[string]interface{}) {
	raw, ok := msg["result"]
	if !ok {
		return
	}
	var rows []map[string]interface{}
	switch v := raw.(type) {
	case []interface{}:
		for _, e := range v {
			row, ok := e.(map[string]interface{})
			if !ok {
				continue
			}
			rows = append(rows, row)
		}
	case map[string]interface{}:
		rows = append(rows, v)
	default:
		return
	}
	for _, row := range rows {
		snap, ok := balanceUpdateFromWSRow(row)
		if !ok {
			continue
		}
		ev, err := perp.NewUserEvent(perp.UserBalanceUpdate, snap)
		if err != nil {
			h.c.log.ErrorContext(h.hubCtx, "ws user event", "err", err)
			continue
		}
		select {
		case h.c.userCh <- ev:
		default:
		}
	}
}

func userTradeFromWSRow(row map[string]interface{}) (*perp.TradeSnapshot, bool) {
	contract := perp.CanonicalContract(str(row["contract"]))
	if contract == "" {
		return nil, false
	}
	tradeID := str(row["id"])
	if tradeID == "" {
		return nil, false
	}
	sizeRaw := str(row["size"])
	if sizeRaw == "" {
		return nil, false
	}
	ts := parseFloat(str(row["create_time_ms"]))
	if ts == 0 {
		ts = float64(int64Val(row["create_time"])) * 1000
	}
	var createdAt time.Time
	if ts > 0 {
		createdAt = futuresTime(ts)
	} else {
		createdAt = time.Now()
	}
	fee := str(row["fee"])
	if fee == "" {
		fee = "0"
	}
	pointFee := str(row["point_fee"])
	if pointFee == "" {
		pointFee = "0"
	}
	return &perp.TradeSnapshot{
		TradeID:         tradeID,
		ExchangeOrderID: str(row["order_id"]),
		Contract:        contract,
		Side:            gateSizeSide(sizeRaw),
		Size:            trimFloatString(math.Abs(parseFloat(sizeRaw))),
		Price:           str(row["price"]),
		Role:            str(row["role"]),
		Fee:             fee,
		PointFee:        pointFee,
		Text:            str(row["text"]),
		CreatedAt:       createdAt,
	}, true
}

func positionFromWSRow(row map[string]interface{}) (*perp.PositionSnapshot, bool) {
	contract := perp.CanonicalContract(str(row["contract"]))
	if contract == "" {
		return nil, false
	}
	sizeRaw := str(row["size"])
	sz := parseFloat(sizeRaw)
	side := perp.PositionLong
	if sz < 0 {
		side = perp.PositionShort
	}
	ts := parseFloat(str(row["time_ms"]))
	if ts == 0 {
		ts = float64(int64Val(row["time"]))
		if ts > 0 && ts < 1_000_000_000_000 {
			ts *= 1000
		}
	}
	var updatedAt time.Time
	if ts > 0 {
		updatedAt = futuresTime(ts)
	} else {
		updatedAt = time.Now()
	}
	unrealized := str(row["unrealised_pnl"])
	if unrealized == "" {
		unrealized = str(row["unrealized_pnl"])
	}
	return &perp.PositionSnapshot{
		Contract:      contract,
		Side:          side,
		Size:          trimFloatString(math.Abs(sz)),
		EntryPrice:    str(row["entry_price"]),
		MarkPrice:     str(row["mark_price"]),
		UnrealizedPnl: unrealized,
		Leverage:      int(int64Val(row["leverage"])),
		UpdatedAt:     updatedAt,
	}, true
}

func (h *wsHub) handlePositions(msg map[string]interface{}) {
	raw, ok := msg["result"]
	if !ok {
		return
	}
	var rows []map[string]interface{}
	switch v := raw.(type) {
	case []interface{}:
		for _, e := range v {
			row, ok := e.(map[string]interface{})
			if !ok {
				continue
			}
			rows = append(rows, row)
		}
	case map[string]interface{}:
		rows = append(rows, v)
	default:
		return
	}
	for _, row := range rows {
		snap, ok := positionFromWSRow(row)
		if !ok {
			continue
		}
		ev, err := perp.NewUserEvent(perp.UserPositionUpdate, snap)
		if err != nil {
			h.c.log.ErrorContext(h.hubCtx, "ws user event", "err", err)
			continue
		}
		select {
		case h.c.userCh <- ev:
		default:
		}
	}
}

func (h *wsHub) handleUserTrades(msg map[string]interface{}) {
	raw, ok := msg["result"]
	if !ok {
		return
	}
	var rows []map[string]interface{}
	switch v := raw.(type) {
	case []interface{}:
		for _, e := range v {
			row, ok := e.(map[string]interface{})
			if !ok {
				continue
			}
			rows = append(rows, row)
		}
	case map[string]interface{}:
		rows = append(rows, v)
	default:
		return
	}
	for _, row := range rows {
		snap, ok := userTradeFromWSRow(row)
		if !ok {
			continue
		}
		ev, err := perp.NewUserEvent(perp.UserTradeUpdate, snap)
		if err != nil {
			h.c.log.ErrorContext(h.hubCtx, "ws user event", "err", err)
			continue
		}
		select {
		case h.c.userCh <- ev:
		default:
		}
	}
}

func (h *wsHub) handleOrder(msg map[string]interface{}) {
	raw, ok := msg["result"]
	if !ok {
		return
	}
	var orders []gateapi.FuturesOrder
	switch v := raw.(type) {
	case []interface{}:
		for _, e := range v {
			fo, ok := futuresOrderFromWSElement(e)
			if !ok {
				continue
			}
			orders = append(orders, fo)
		}
	case map[string]interface{}:
		fo, ok := futuresOrderFromWSElement(v)
		if ok {
			orders = append(orders, fo)
		}
	default:
		return
	}
	for _, fo := range orders {
		ov := orderViewFromFutures(fo)
		ev, err := perp.NewUserEvent(perp.UserOrderUpdate, ov)
		if err != nil {
			h.c.log.ErrorContext(h.hubCtx, "ws user event", "err", err)
			continue
		}
		select {
		case h.c.userCh <- ev:
		default:
		}
	}
}
