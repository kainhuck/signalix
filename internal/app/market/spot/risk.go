package spot

import (
	"context"
	"fmt"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
	spotex "github.com/kainhuck/signalix/pkg/exchange/spot"
	"github.com/shopspring/decimal"
)

type SpotRiskOMS interface {
	NonFinalOrderCount() int
}

type SpotRiskEquity interface {
	DailyLoss() decimal.Decimal
	DrawdownRatio() decimal.Decimal
}

type SpotRiskConfig struct {
	Projection *AccountProjection
	Router     *MarketRouter
	Execution  SpotRiskOMS
	Equity     SpotRiskEquity
}

type SpotRisk struct {
	proj      *AccountProjection
	router    *MarketRouter
	execution SpotRiskOMS
	equity    SpotRiskEquity
}

func NewSpotRisk(cfg SpotRiskConfig) market.MarketRisk {
	return &SpotRisk{
		proj:      cfg.Projection,
		router:    cfg.Router,
		execution: cfg.Execution,
		equity:    cfg.Equity,
	}
}

func (s *SpotRisk) BuildRiskContext(ctx context.Context, strategy string, sig *models.Signal, o *models.Order) (*ports.RiskContext, error) {
	_ = ctx
	if s == nil || s.proj == nil {
		return nil, fmt.Errorf("spot risk not configured")
	}
	if o == nil {
		return nil, fmt.Errorf("nil order")
	}
	pair := spotex.CanonicalPair(string(o.Symbol))
	if pair == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	rc := &ports.RiskContext{
		StrategyName:       strategy,
		Signal:             sig,
		Order:              o,
		QuoteCcy:           pair.QuoteCurrency(),
		ProjectionReady:    s.proj.IsReady(),
		OpensExposure:      o.Side == models.OrderSideBuy,
		IncreasingExposure: o.Side == models.OrderSideBuy,
		NotionalAvailable:  true,
	}
	if s.execution != nil {
		rc.OpenOrders = s.execution.NonFinalOrderCount()
	}
	rc.Positions = len(s.proj.ListPositions())
	if s.equity != nil {
		rc.DailyLoss = s.equity.DailyLoss()
		rc.DrawdownRatio = s.equity.DrawdownRatio()
	}

	orderNotional, unitPrice, err := s.orderNotional(pair, o)
	if err != nil {
		rc.NotionalAvailable = false
		return rc, err
	}
	if err := s.checkBalance(pair, o, orderNotional); err != nil {
		return rc, err
	}

	positionNotional := s.positionNotional(pair)
	totalExposure := s.totalExposure()
	rc.OrderNotional = orderNotional
	rc.PositionNotional = positionNotional
	rc.TotalExposure = totalExposure
	rc.AccountEquity = s.accountEquity()
	rc.NotionalPerUnit = unitPrice
	if rc.NotionalPerUnit.Sign() <= 0 {
		rc.NotionalPerUnit = decimal.NewFromInt(1)
	}
	if o.Side == models.OrderSideBuy {
		rc.PostPositionNotional = positionNotional.Add(orderNotional)
		rc.PostTotalExposure = totalExposure.Add(orderNotional)
	} else {
		rc.PostPositionNotional = decimalMax(positionNotional.Sub(orderNotional), decimal.Zero)
		rc.PostTotalExposure = decimalMax(totalExposure.Sub(orderNotional), decimal.Zero)
	}
	return rc, nil
}

func (s *SpotRisk) checkBalance(pair spotex.Pair, o *models.Order, orderNotional decimal.Decimal) error {
	switch o.Side {
	case models.OrderSideBuy:
		available := parseDecimal(s.proj.Balance(pair.QuoteCurrency()).Available)
		if available.LessThan(orderNotional) {
			return fmt.Errorf("quote available %s below order notional %s", available, orderNotional)
		}
	case models.OrderSideSell:
		available := parseDecimal(s.proj.Balance(pair.BaseCurrency()).Available)
		size := parseDecimal(o.Size)
		if available.LessThan(size) {
			return fmt.Errorf("base available %s below order size %s", available, size)
		}
	}
	return nil
}

func (s *SpotRisk) orderNotional(pair spotex.Pair, o *models.Order) (decimal.Decimal, decimal.Decimal, error) {
	size, err := positiveDecimal(o.Size, "order size")
	if err != nil {
		return decimal.Zero, decimal.Zero, err
	}
	switch o.OrderType {
	case models.OrderTypeMarket:
		if o.Side == models.OrderSideBuy {
			return size, decimal.NewFromInt(1), nil
		}
		price, err := s.priceForPair(pair)
		if err != nil {
			return decimal.Zero, decimal.Zero, err
		}
		return size.Mul(price), price, nil
	case models.OrderTypeLimit:
		if o.Price == nil {
			return decimal.Zero, decimal.Zero, fmt.Errorf("limit order requires price")
		}
		price, err := positiveDecimal(*o.Price, "limit price")
		if err != nil {
			return decimal.Zero, decimal.Zero, err
		}
		return size.Mul(price), price, nil
	default:
		return decimal.Zero, decimal.Zero, fmt.Errorf("unsupported order type: %s", o.OrderType)
	}
}

func (s *SpotRisk) positionNotional(pair spotex.Pair) decimal.Decimal {
	baseTotal := parseDecimal(s.proj.Balance(pair.BaseCurrency()).Total)
	if !baseTotal.IsPositive() {
		return decimal.Zero
	}
	price, err := s.priceForPair(pair)
	if err != nil {
		return decimal.Zero
	}
	return baseTotal.Mul(price)
}

func (s *SpotRisk) totalExposure() decimal.Decimal {
	total := decimal.Zero
	seen := make(map[string]bool)
	for pair := range s.proj.pairMeta {
		base := pair.BaseCurrency()
		if base == "" || seen[base] {
			continue
		}
		seen[base] = true
		total = total.Add(s.positionNotional(pair))
	}
	return total
}

func (s *SpotRisk) accountEquity() decimal.Decimal {
	balances := s.proj.Balances()
	total := decimal.Zero
	seenBase := make(map[string]bool)
	for ccy, bal := range balances {
		if bal == nil {
			continue
		}
		if ccy == "USDT" {
			total = total.Add(parseDecimal(bal.Total))
			continue
		}
		seenBase[ccy] = true
	}
	for pair := range s.proj.pairMeta {
		base := pair.BaseCurrency()
		if !seenBase[base] {
			continue
		}
		seenBase[base] = false
		total = total.Add(s.positionNotional(pair))
	}
	return total
}

func (s *SpotRisk) priceForPair(pair spotex.Pair) (decimal.Decimal, error) {
	if s == nil || s.router == nil {
		return decimal.Zero, fmt.Errorf("spot ticker lookup not configured")
	}
	ticker, ok := s.router.GetCachedTicker(pair)
	if !ok || ticker == nil {
		return decimal.Zero, fmt.Errorf("no cached ticker for %s", pair)
	}
	for _, raw := range []string{ticker.Last} {
		price := parseDecimal(raw)
		if price.IsPositive() {
			return price, nil
		}
	}
	return decimal.Zero, fmt.Errorf("no positive ticker price for %s", pair)
}

func decimalMax(a, b decimal.Decimal) decimal.Decimal {
	if a.GreaterThan(b) {
		return a
	}
	return b
}

var _ market.MarketRisk = (*SpotRisk)(nil)
