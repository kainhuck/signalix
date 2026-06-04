package gateio

import (
	"strings"
	"time"

	"github.com/kainhuck/signalix/pkg/exchange/spot"

	"github.com/gate/gateapi-go/v7"
)

func (h *wsHub) handleTicker(msg map[string]interface{}) {
	raw, ok := msg["result"]
	if !ok {
		return
	}
	row, ok := raw.(map[string]interface{})
	if !ok {
		return
	}
	ts := wsJSONInt64(msg["time_ms"])
	if ts == 0 {
		ts = wsJSONInt64(msg["time"]) * 1000
	}
	if ts == 0 {
		ts = time.Now().UnixMilli()
	}
	snap := &spot.TickerSnapshot{
		Pair:            spot.CanonicalPair(str(row["currency_pair"])),
		Last:            str(row["last"]),
		HighestBid:      str(row["highest_bid"]),
		LowestAsk:       str(row["lowest_ask"]),
		ChangePct24h:    strPct(str(row["change_percentage"])),
		Volume24hBase:   str(row["base_volume"]),
		Volume24hQuote:  str(row["quote_volume"]),
		High24h:         str(row["high_24h"]),
		Low24h:          str(row["low_24h"]),
		TimestampMillis: ts,
	}
	ev, err := spot.NewPublicEvent(spot.PublicTicker, snap)
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

func boolVal(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	default:
		return false
	}
}

func parseSpotCandlestickName(n string) (interval string, pair spot.Pair) {
	n = strings.TrimSpace(n)
	i := strings.Index(n, "_")
	if i <= 0 || i >= len(n)-1 {
		return "", ""
	}
	return n[:i], spot.CanonicalPair(n[i+1:])
}

func candlestickFromWSRow(row map[string]interface{}) (*spot.CandlestickSnapshot, bool) {
	interval, pair := parseSpotCandlestickName(str(row["n"]))
	if pair == "" {
		return nil, false
	}
	ts := wsJSONInt64(row["t"])
	if ts <= 0 {
		return nil, false
	}
	return &spot.CandlestickSnapshot{
		Pair:         pair,
		Interval:     interval,
		Open:         str(row["o"]),
		High:         str(row["h"]),
		Low:          str(row["l"]),
		Close:        str(row["c"]),
		Volume:       str(row["v"]),
		VolumeBase:   str(row["a"]),
		TimestampSec: ts,
		WindowClosed: boolVal(row["w"]),
	}, true
}

func (h *wsHub) handleCandlestick(msg map[string]interface{}) {
	raw, ok := msg["result"]
	if !ok {
		return
	}
	row, ok := raw.(map[string]interface{})
	if !ok {
		return
	}
	snap, ok := candlestickFromWSRow(row)
	if !ok {
		return
	}
	ev, err := spot.NewPublicEvent(spot.PublicCandlestick, snap)
	if err != nil {
		h.c.log.ErrorContext(h.hubCtx, "ws public event", "err", err)
		return
	}
	select {
	case h.c.pubCh <- ev:
	default:
	}
}

func balanceUpdateFromWSRow(row map[string]interface{}) (*spot.BalanceUpdateSnapshot, bool) {
	currency := strings.ToUpper(str(row["currency"]))
	if currency == "" {
		return nil, false
	}
	ts := wsJSONInt64(row["timestamp_ms"])
	if ts == 0 {
		ts = wsJSONInt64(row["timestamp"]) * 1000
	}
	var updatedAt time.Time
	if ts > 0 {
		updatedAt = time.UnixMilli(ts)
	} else {
		updatedAt = time.Now()
	}
	total := str(row["total"])
	if total == "" {
		avail := parseFloat(str(row["available"]))
		frozen := parseFloat(str(row["freeze"]))
		if frozen == 0 {
			frozen = parseFloat(str(row["locked"]))
		}
		total = trimFloatString(avail + frozen)
	}
	frozen := str(row["freeze"])
	if frozen == "" {
		frozen = str(row["locked"])
	}
	return &spot.BalanceUpdateSnapshot{
		Currency:   currency,
		Total:      total,
		Available:  str(row["available"]),
		Frozen:     frozen,
		Change:     str(row["change"]),
		ChangeType: str(row["change_type"]),
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
		ev, err := spot.NewUserEvent(spot.UserBalanceUpdate, snap)
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
	var orders []gateapi.Order
	switch v := raw.(type) {
	case []interface{}:
		for _, e := range v {
			o, ok := spotOrderFromWSElement(e)
			if !ok {
				continue
			}
			orders = append(orders, o)
		}
	case map[string]interface{}:
		o, ok := spotOrderFromWSElement(v)
		if ok {
			orders = append(orders, o)
		}
	default:
		return
	}
	for _, o := range orders {
		ov := orderViewFromSpot(o)
		ev, err := spot.NewUserEvent(spot.UserOrderUpdate, ov)
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
