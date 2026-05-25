package gateio

import (
	"strings"

	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

// Gate 永续 WebSocket 频道名（与官方文档一致）。
const (
	WSChannelTickers          = "futures.tickers"
	WSChannelTrades           = "futures.trades"
	WSChannelOrderBook        = "futures.order_book"
	WSChannelOrderBookUpdate  = "futures.order_book_update"
	WSChannelBookTicker       = "futures.book_ticker"
	WSChannelOBU              = "futures.obu"
	WSChannelCandlesticks     = "futures.candlesticks"
	WSChannelPublicLiquidates = "futures.public_liquidates"
	WSChannelContractStats    = "futures.contract_stats"
	WSChannelOrders           = "futures.orders"
	WSChannelUserTrades       = "futures.usertrades"
	WSChannelLiquidates       = "futures.liquidates"
	WSChannelAutoDeleverages  = "futures.auto_deleverages"
	WSChannelPositionCloses   = "futures.position_closes"
	WSChannelBalances         = "futures.balances"
	WSChannelReduceRiskLimits = "futures.reduce_risk_limits"
	WSChannelPositions        = "futures.positions"
	WSChannelPositionADLRank  = "futures.position_adl_rank"
	WSChannelAutoOrders       = "futures.autoorders"
)

type wsScope uint8

const (
	wsScopePublic wsScope = iota
	wsScopePrivate
)

type payloadExpand func(h *wsHub, sub *perp.Subscription) ([][]string, error)

type wsChannelSpec struct {
	name     string
	scope    wsScope
	expand   payloadExpand
	onUpdate func(h *wsHub, msg map[string]interface{})
}

func wsSubKey(channel string, payload []string) string {
	return channel + "|" + strings.Join(payload, ",")
}

func normalizeWSChannel(ch string) string {
	ch = strings.TrimSpace(ch)
	if ch == "" {
		return WSChannelTickers
	}
	return ch
}

func wsChannelSpecFor(name string) (*wsChannelSpec, bool) {
	spec, ok := wsChannelRegistry[name]
	return spec, ok
}

var wsChannelRegistry = map[string]*wsChannelSpec{
	WSChannelTickers:          {name: WSChannelTickers, scope: wsScopePublic, expand: expandContractOnly, onUpdate: (*wsHub).handleTicker},
	WSChannelTrades:           {name: WSChannelTrades, scope: wsScopePublic, expand: expandContractOnly},
	WSChannelOrderBook:        {name: WSChannelOrderBook, scope: wsScopePublic, expand: expandOrderBook},
	WSChannelOrderBookUpdate:  {name: WSChannelOrderBookUpdate, scope: wsScopePublic, expand: expandOrderBookUpdate},
	WSChannelBookTicker:       {name: WSChannelBookTicker, scope: wsScopePublic, expand: expandContractOnly},
	WSChannelOBU:              {name: WSChannelOBU, scope: wsScopePublic, expand: expandRawPayload},
	WSChannelCandlesticks:     {name: WSChannelCandlesticks, scope: wsScopePublic, expand: expandCandlesticks, onUpdate: (*wsHub).handleCandlestick},
	WSChannelPublicLiquidates: {name: WSChannelPublicLiquidates, scope: wsScopePublic, expand: expandContractOnly},
	WSChannelContractStats:    {name: WSChannelContractStats, scope: wsScopePublic, expand: expandContractStats},

	WSChannelOrders:           {name: WSChannelOrders, scope: wsScopePrivate, expand: expandUserContract, onUpdate: (*wsHub).handleOrder},
	WSChannelUserTrades:       {name: WSChannelUserTrades, scope: wsScopePrivate, expand: expandUserContract, onUpdate: (*wsHub).handleUserTrades},
	WSChannelLiquidates:       {name: WSChannelLiquidates, scope: wsScopePrivate, expand: expandUserContract},
	WSChannelAutoDeleverages:  {name: WSChannelAutoDeleverages, scope: wsScopePrivate, expand: expandUserContract},
	WSChannelPositionCloses:   {name: WSChannelPositionCloses, scope: wsScopePrivate, expand: expandUserContract},
	WSChannelBalances:         {name: WSChannelBalances, scope: wsScopePrivate, expand: expandUserOnly, onUpdate: (*wsHub).handleBalances},
	WSChannelReduceRiskLimits: {name: WSChannelReduceRiskLimits, scope: wsScopePrivate, expand: expandUserContract},
	WSChannelPositions:        {name: WSChannelPositions, scope: wsScopePrivate, expand: expandUserContract, onUpdate: (*wsHub).handlePositions},
	WSChannelPositionADLRank:  {name: WSChannelPositionADLRank, scope: wsScopePrivate, expand: expandContractAuth},
	WSChannelAutoOrders:       {name: WSChannelAutoOrders, scope: wsScopePrivate, expand: expandContractAuth},
}

func expandContractOnly(_ *wsHub, sub *perp.Subscription) ([][]string, error) {
	if len(sub.Contracts) == 0 {
		return nil, perp.NewError(perp.ErrInvalidParameter, "contracts required for "+sub.Channel, nil)
	}
	out := make([][]string, 0, len(sub.Contracts))
	for _, ct := range sub.Contracts {
		out = append(out, []string{toGateContract(perp.CanonicalContract(string(ct)))})
	}
	return out, nil
}

func expandContractAuth(_ *wsHub, sub *perp.Subscription) ([][]string, error) {
	return expandContractOnly(nil, sub)
}

func expandUserOnly(h *wsHub, _ *perp.Subscription) ([][]string, error) {
	uid := strings.TrimSpace(h.c.userID)
	if uid == "" {
		return nil, perp.NewError(perp.ErrInvalidParameter, "private channel requires gateio.WithUserID", nil)
	}
	return [][]string{{uid}}, nil
}

func expandUserContract(h *wsHub, sub *perp.Subscription) ([][]string, error) {
	uid := strings.TrimSpace(h.c.userID)
	if uid == "" {
		return nil, perp.NewError(perp.ErrInvalidParameter, "private channel requires gateio.WithUserID", nil)
	}
	if len(sub.Contracts) == 0 {
		return [][]string{{uid, "!all"}}, nil
	}
	out := make([][]string, 0, len(sub.Contracts))
	for _, ct := range sub.Contracts {
		out = append(out, []string{uid, toGateContract(perp.CanonicalContract(string(ct)))})
	}
	return out, nil
}

func expandRawPayload(_ *wsHub, sub *perp.Subscription) ([][]string, error) {
	if len(sub.Payload) == 0 {
		return nil, perp.NewError(perp.ErrInvalidParameter, sub.Channel+" requires Payload", nil)
	}
	cp := append([]string(nil), sub.Payload...)
	return [][]string{cp}, nil
}

func expandCandlesticks(_ *wsHub, sub *perp.Subscription) ([][]string, error) {
	interval := ""
	if len(sub.Payload) > 0 {
		interval = strings.TrimSpace(sub.Payload[0])
	}
	if interval == "" {
		return nil, perp.NewError(perp.ErrInvalidParameter, WSChannelCandlesticks+" requires interval in Payload[0]", nil)
	}
	if len(sub.Contracts) == 0 {
		return nil, perp.NewError(perp.ErrInvalidParameter, WSChannelCandlesticks+" requires contract", nil)
	}
	out := make([][]string, 0, len(sub.Contracts))
	for _, ct := range sub.Contracts {
		out = append(out, []string{interval, toGateContract(perp.CanonicalContract(string(ct)))})
	}
	return out, nil
}

func expandOrderBook(_ *wsHub, sub *perp.Subscription) ([][]string, error) {
	limit := "20"
	interval := "0"
	if len(sub.Payload) > 0 && strings.TrimSpace(sub.Payload[0]) != "" {
		limit = strings.TrimSpace(sub.Payload[0])
	}
	if len(sub.Payload) > 1 && strings.TrimSpace(sub.Payload[1]) != "" {
		interval = strings.TrimSpace(sub.Payload[1])
	}
	if len(sub.Contracts) == 0 {
		return nil, perp.NewError(perp.ErrInvalidParameter, WSChannelOrderBook+" requires contract", nil)
	}
	out := make([][]string, 0, len(sub.Contracts))
	for _, ct := range sub.Contracts {
		out = append(out, []string{
			toGateContract(perp.CanonicalContract(string(ct))),
			limit,
			interval,
		})
	}
	return out, nil
}

func expandOrderBookUpdate(_ *wsHub, sub *perp.Subscription) ([][]string, error) {
	frequency := "100ms"
	level := "100"
	if len(sub.Payload) > 0 && strings.TrimSpace(sub.Payload[0]) != "" {
		frequency = strings.TrimSpace(sub.Payload[0])
	}
	if len(sub.Payload) > 1 && strings.TrimSpace(sub.Payload[1]) != "" {
		level = strings.TrimSpace(sub.Payload[1])
	}
	if len(sub.Contracts) == 0 {
		return nil, perp.NewError(perp.ErrInvalidParameter, WSChannelOrderBookUpdate+" requires contract", nil)
	}
	out := make([][]string, 0, len(sub.Contracts))
	for _, ct := range sub.Contracts {
		out = append(out, []string{
			toGateContract(perp.CanonicalContract(string(ct))),
			frequency,
			level,
		})
	}
	return out, nil
}

func expandContractStats(_ *wsHub, sub *perp.Subscription) ([][]string, error) {
	interval := "1m"
	if len(sub.Payload) > 0 && strings.TrimSpace(sub.Payload[0]) != "" {
		interval = strings.TrimSpace(sub.Payload[0])
	}
	if len(sub.Contracts) == 0 {
		return nil, perp.NewError(perp.ErrInvalidParameter, WSChannelContractStats+" requires contract", nil)
	}
	out := make([][]string, 0, len(sub.Contracts))
	for _, ct := range sub.Contracts {
		out = append(out, []string{
			toGateContract(perp.CanonicalContract(string(ct))),
			interval,
		})
	}
	return out, nil
}
