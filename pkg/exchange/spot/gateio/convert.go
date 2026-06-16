package gateio

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/kainhuck/signalix/pkg/exchange/spot"

	"github.com/gate/gateapi-go/v7"
)

func toGatePair(p spot.Pair) string {
	s := string(p.Canonical())
	if s == "" {
		return ""
	}
	return strings.ReplaceAll(s, "/", "_")
}

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

func trimFloatString(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	if s == "" || s == "-" {
		return "0"
	}
	return s
}

func mapSide(s string) spot.Side {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "buy":
		return spot.SideBuy
	case "sell":
		return spot.SideSell
	default:
		return spot.Side("")
	}
}

func mapGateSide(s spot.Side) string {
	switch s {
	case spot.SideBuy:
		return "buy"
	case spot.SideSell:
		return "sell"
	default:
		return ""
	}
}

func mapTIF(tif spot.TimeInForce) string {
	switch tif {
	case spot.TIFIOC:
		return "ioc"
	case spot.TIFFOK:
		return "fok"
	case spot.TIFPOC:
		return "poc"
	default:
		return "gtc"
	}
}

func orderTypeFromGate(o gateapi.Order) spot.OrderType {
	if strings.EqualFold(o.Type, "market") {
		return spot.OrderTypeMarket
	}
	return spot.OrderTypeLimit
}

func spotFilled(o gateapi.Order) float64 {
	if fs := strings.TrimSpace(o.FilledAmount); fs != "" {
		return parseFloat(fs)
	}
	total := parseFloat(o.Amount)
	left := parseFloat(o.Left)
	if total > 0 && left >= 0 {
		return math.Max(0, total-left)
	}
	return 0
}

func mapSpotOrderStatus(o gateapi.Order) spot.OrderStatus {
	switch strings.ToLower(strings.TrimSpace(o.Status)) {
	case "open":
		left := parseFloat(o.Left)
		total := parseFloat(o.Amount)
		if total > 0 && left > 0 && left < total {
			return spot.OrderPartialFilled
		}
		return spot.OrderSubmitted
	case "closed":
		if spotFilled(o) > 0 && parseFloat(o.Left) == 0 {
			return spot.OrderFilled
		}
		return spot.OrderFilled
	case "cancelled":
		return spot.OrderCancelled
	default:
		return spot.OrderRejected
	}
}

func spotOrderTime(o gateapi.Order) time.Time {
	if o.UpdateTimeMs > 0 {
		return time.UnixMilli(o.UpdateTimeMs)
	}
	if o.CreateTimeMs > 0 {
		return time.UnixMilli(o.CreateTimeMs)
	}
	if ts := parseFloat(o.UpdateTime); ts > 0 {
		return unixFromFloat(ts)
	}
	if ts := parseFloat(o.CreateTime); ts > 0 {
		return unixFromFloat(ts)
	}
	return time.Time{}
}

func unixFromFloat(ts float64) time.Time {
	if ts >= 1_000_000_000_000 {
		return time.UnixMilli(int64(ts))
	}
	sec := int64(ts)
	nsec := int64((ts - float64(sec)) * 1e9)
	return time.Unix(sec, nsec)
}

func orderViewFromSpot(o gateapi.Order) *spot.OrderSnapshot {
	price := strings.TrimSpace(o.Price)
	if parseFloat(price) == 0 {
		price = ""
	}
	avg := strings.TrimSpace(o.AvgDealPrice)
	if parseFloat(avg) == 0 {
		avg = ""
	}
	created := spotOrderTime(o)
	updated := created
	if o.UpdateTimeMs > 0 || strings.TrimSpace(o.UpdateTime) != "" {
		updated = spotOrderTime(o)
	}
	return &spot.OrderSnapshot{
		OrderID:         strings.TrimSpace(o.Id),
		ExchangeOrderID: strings.TrimSpace(o.Id),
		ClientID:        strings.TrimSpace(o.Text),
		Pair:            spot.CanonicalPair(o.CurrencyPair),
		Side:            mapSide(o.Side),
		Type:            orderTypeFromGate(o),
		Size:            strings.TrimSpace(o.Amount),
		Price:           price,
		FilledSize:      trimFloatString(spotFilled(o)),
		AvgPrice:        avg,
		Status:          mapSpotOrderStatus(o),
		CreatedAt:       created,
		UpdatedAt:       updated,
	}
}

func pairMetaFromGate(cp gateapi.CurrencyPair) *spot.PairMeta {
	id := strings.TrimSpace(cp.Id)
	if id == "" {
		return nil
	}
	return &spot.PairMeta{
		Pair:            spot.CanonicalPair(id),
		MinBaseAmount:   strings.TrimSpace(cp.MinBaseAmount),
		MinQuoteAmount:  strings.TrimSpace(cp.MinQuoteAmount),
		MaxBaseAmount:   strings.TrimSpace(cp.MaxBaseAmount),
		AmountPrecision: cp.AmountPrecision,
		PricePrecision:  cp.Precision,
		TradeStatus:     strings.TrimSpace(cp.TradeStatus),
	}
}

func balanceViewFromSpot(ac gateapi.SpotAccount) *spot.BalanceView {
	avail := parseFloat(ac.Available)
	locked := parseFloat(ac.Locked)
	total := avail + locked
	return &spot.BalanceView{
		Currency:  strings.ToUpper(strings.TrimSpace(ac.Currency)),
		Total:     trimFloatString(total),
		Available: trimFloatString(avail),
		Frozen:    trimFloatString(locked),
		UpdatedAt: time.Now(),
	}
}

const gateOrderTextMaxLen = 30

func normalizeClientOrderID(id string) string {
	raw := strings.TrimSpace(id)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "t-") {
		raw = strings.TrimPrefix(raw, "t-")
	}
	const bodyMax = gateOrderTextMaxLen - 2
	if len(raw) > bodyMax {
		sum := sha256.Sum256([]byte(raw))
		raw = hex.EncodeToString(sum[:])[:bodyMax]
	}
	return "t-" + raw
}

func buildGateOrder(req *spot.PlaceRequest) (gateapi.Order, error) {
	if req == nil {
		return gateapi.Order{}, spot.NewError(spot.ErrInvalidParameter, "request is nil", nil)
	}
	pair := toGatePair(req.Pair)
	if pair == "" {
		return gateapi.Order{}, spot.NewError(spot.ErrInvalidParameter, "pair is required", nil)
	}
	side := mapGateSide(req.Side)
	if side == "" {
		return gateapi.Order{}, spot.NewError(spot.ErrInvalidParameter, "invalid side", nil)
	}

	acct := strings.TrimSpace(req.Account)
	if acct == "" {
		acct = "spot"
	}
	if acct != "spot" {
		return gateapi.Order{}, spot.NewError(spot.ErrNotSupported, "only spot account is supported", nil)
	}

	var amount string
	var typ string
	switch req.Type {
	case spot.OrderTypeLimit:
		typ = "limit"
		if req.Price == nil || parseFloat(*req.Price) <= 0 {
			return gateapi.Order{}, spot.NewError(spot.ErrInvalidParameter, "limit order requires positive price", nil)
		}
		if parseFloat(req.Size) <= 0 {
			return gateapi.Order{}, spot.NewError(spot.ErrInvalidParameter, "size must be positive", nil)
		}
		amount = strings.TrimSpace(req.Size)
	case spot.OrderTypeMarket:
		typ = "market"
		switch req.Side {
		case spot.SideBuy:
			if req.QuoteAmount == nil || parseFloat(*req.QuoteAmount) <= 0 {
				return gateapi.Order{}, spot.NewError(spot.ErrInvalidParameter, "market buy requires positive quote_amount", nil)
			}
			amount = strings.TrimSpace(*req.QuoteAmount)
		case spot.SideSell:
			if parseFloat(req.Size) <= 0 {
				return gateapi.Order{}, spot.NewError(spot.ErrInvalidParameter, "size must be positive", nil)
			}
			amount = strings.TrimSpace(req.Size)
		default:
			return gateapi.Order{}, spot.NewError(spot.ErrInvalidParameter, "invalid side", nil)
		}
	default:
		return gateapi.Order{}, spot.NewError(spot.ErrNotSupported, "only Market and Limit", nil)
	}

	o := gateapi.Order{
		CurrencyPair: pair,
		Side:         side,
		Type:         typ,
		Amount:       amount,
		Account:      acct,
	}
	if req.Type == spot.OrderTypeLimit {
		o.Price = strings.TrimSpace(*req.Price)
		o.TimeInForce = mapTIF(req.TimeInForce)
	} else {
		o.TimeInForce = "ioc"
	}
	if cid := normalizeClientOrderID(req.ClientID); cid != "" {
		o.Text = cid
	}
	return o, nil
}
