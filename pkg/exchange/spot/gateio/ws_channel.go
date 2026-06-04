package gateio

import (
	"strings"

	"github.com/kainhuck/signalix/pkg/exchange/spot"
)

const (
	WSChannelTickers      = "spot.tickers"
	WSChannelCandlesticks = "spot.candlesticks"
	WSChannelOrders       = "spot.orders"
	WSChannelBalances     = "spot.balances"
)

type wsScope uint8

const (
	wsScopePublic wsScope = iota
	wsScopePrivate
)

type payloadExpand func(h *wsHub, sub *spot.Subscription) ([][]string, error)

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
	WSChannelTickers: {
		name: WSChannelTickers, scope: wsScopePublic,
		expand: expandPairOnly, onUpdate: (*wsHub).handleTicker,
	},
	WSChannelCandlesticks: {
		name: WSChannelCandlesticks, scope: wsScopePublic,
		expand: expandCandlesticks, onUpdate: (*wsHub).handleCandlestick,
	},
	WSChannelOrders: {
		name: WSChannelOrders, scope: wsScopePrivate,
		expand: expandPairOnly, onUpdate: (*wsHub).handleOrder,
	},
	WSChannelBalances: {
		name: WSChannelBalances, scope: wsScopePrivate,
		expand: expandEmptyPayload, onUpdate: (*wsHub).handleBalances,
	},
}

func expandPairOnly(_ *wsHub, sub *spot.Subscription) ([][]string, error) {
	if len(sub.Pairs) == 0 {
		return nil, spot.NewError(spot.ErrInvalidParameter, "pairs required for "+sub.Channel, nil)
	}
	out := make([][]string, 0, len(sub.Pairs))
	for _, p := range sub.Pairs {
		gp := toGatePair(p.Canonical())
		if gp == "" {
			return nil, spot.NewError(spot.ErrInvalidParameter, "invalid pair", nil)
		}
		out = append(out, []string{gp})
	}
	return out, nil
}

func expandEmptyPayload(_ *wsHub, _ *spot.Subscription) ([][]string, error) {
	return [][]string{{}}, nil
}

func expandCandlesticks(_ *wsHub, sub *spot.Subscription) ([][]string, error) {
	interval := ""
	if len(sub.Payload) > 0 {
		interval = strings.TrimSpace(sub.Payload[0])
	}
	if interval == "" {
		return nil, spot.NewError(spot.ErrInvalidParameter, WSChannelCandlesticks+" requires interval in Payload[0]", nil)
	}
	if len(sub.Pairs) == 0 {
		return nil, spot.NewError(spot.ErrInvalidParameter, WSChannelCandlesticks+" requires pair", nil)
	}
	out := make([][]string, 0, len(sub.Pairs))
	for _, p := range sub.Pairs {
		gp := toGatePair(p.Canonical())
		if gp == "" {
			return nil, spot.NewError(spot.ErrInvalidParameter, "invalid pair", nil)
		}
		out = append(out, []string{interval, gp})
	}
	return out, nil
}
