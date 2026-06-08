package spot

import (
	"context"
	"fmt"
	"time"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
	"github.com/kainhuck/signalix/pkg/exchange/spot"
	"github.com/shopspring/decimal"
)

type SpotDecider struct {
	exchange ports.SpotExchange
	router   *SpotRouter
	divisor  int
}

func NewSpotDecider(exchange ports.SpotExchange, router *SpotRouter, divisor int) market.MarketDecider {
	if divisor <= 0 {
		divisor = 10
	}
	return &SpotDecider{
		exchange: exchange,
		router:   router,
		divisor:  divisor,
	}
}

func (d *SpotDecider) Decide(ctx context.Context, strategy string, sig *models.Signal) (*models.Order, error) {
	if d == nil || d.exchange == nil {
		return nil, fmt.Errorf("spot decider not configured")
	}
	if sig == nil {
		return nil, fmt.Errorf("nil signal")
	}
	if err := sig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid signal: %w", err)
	}

	pair := spot.CanonicalPair(string(sig.Symbol))
	if pair == "" {
		return nil, fmt.Errorf("empty symbol")
	}

	if sig.Direction == models.DirectionFlat {
		return nil, nil
	}

	size, err := d.calculateSize(ctx, pair, sig)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate order size: %w", err)
	}
	if size.IsZero() {
		return nil, nil
	}

	side := spotOrderSide(sig.Direction)
	if side == "" {
		return nil, nil
	}

	order := &models.Order{
		ID:           generateOrderID(),
		Symbol:       sig.Symbol,
		Side:         side,
		OrderType:    models.OrderTypeMarket,
		Size:         size.String(),
		FilledSize:   "0",
		Status:       models.OrderStatusPending,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		StrategyName: strategy,
	}

	if sig.Price != nil {
		price, _ := decimal.NewFromString(*sig.Price)
		if price.IsPositive() {
			order.OrderType = models.OrderTypeLimit
			order.Price = sig.Price
		}
	}

	return order, nil
}

func (d *SpotDecider) calculateSize(ctx context.Context, pair spot.Pair, sig *models.Signal) (decimal.Decimal, error) {
	if sig.SizingMode != nil && *sig.SizingMode == models.SizingModeCustom {
		if sig.Value == nil {
			return decimal.Zero, fmt.Errorf("value is required for custom sizing mode")
		}
		value, _ := decimal.NewFromString(*sig.Value)
		if !value.IsPositive() {
			return decimal.Zero, fmt.Errorf("custom size must be positive")
		}
		return value, nil
	}

	quoteCcy := pair.QuoteCurrency()
	baseCcy := pair.BaseCurrency()

	if sig.Direction == models.DirectionLong {
		return d.calculateBuySize(ctx, pair, quoteCcy, baseCcy)
	}
	return d.calculateSellSize(ctx, pair, baseCcy)
}

func (d *SpotDecider) calculateBuySize(ctx context.Context, pair spot.Pair, quoteCcy, baseCcy string) (decimal.Decimal, error) {
	bv, err := d.exchange.Balance(ctx, quoteCcy)
	if err != nil {
		return decimal.Zero, fmt.Errorf("balance query for %s: %w", quoteCcy, err)
	}
	avail, _ := decimal.NewFromString(bv.Available)
	if avail.Sign() <= 0 {
		return decimal.Zero, fmt.Errorf("insufficient %s balance", quoteCcy)
	}

	divisor := decimal.NewFromInt(int64(d.divisor))
	if divisor.Sign() <= 0 {
		divisor = decimal.NewFromInt(10)
	}
	quoteBudget := avail.Div(divisor)

	last, err := d.pairLastPrice(pair)
	if err != nil {
		return decimal.Zero, err
	}
	if last.Sign() <= 0 {
		return decimal.Zero, fmt.Errorf("invalid price for %s", pair)
	}

	baseQty := quoteBudget.Div(last)
	return baseQty, nil
}

func (d *SpotDecider) calculateSellSize(ctx context.Context, pair spot.Pair, baseCcy string) (decimal.Decimal, error) {
	bv, err := d.exchange.Balance(ctx, baseCcy)
	if err != nil {
		return decimal.Zero, fmt.Errorf("balance query for %s: %w", baseCcy, err)
	}
	avail, _ := decimal.NewFromString(bv.Available)
	if avail.Sign() <= 0 {
		return decimal.Zero, fmt.Errorf("insufficient %s balance", baseCcy)
	}

	divisor := decimal.NewFromInt(int64(d.divisor))
	if divisor.Sign() <= 0 {
		divisor = decimal.NewFromInt(10)
	}
	return avail.Div(divisor), nil
}

func (d *SpotDecider) pairLastPrice(pair spot.Pair) (decimal.Decimal, error) {
	if d.router != nil {
		if snap, ok := d.router.GetCachedTicker(pair); ok && snap != nil {
			last, err := decimal.NewFromString(snap.Last)
			if err == nil && last.IsPositive() {
				return last, nil
			}
		}
	}
	return decimal.Zero, fmt.Errorf("no cached ticker for %s", pair)
}

func spotOrderSide(dir models.Direction) models.OrderSide {
	switch dir {
	case models.DirectionLong:
		return models.OrderSideBuy
	case models.DirectionShort:
		return models.OrderSideSell
	default:
		return ""
	}
}

func generateOrderID() string {
	return fmt.Sprintf("spot-%d", time.Now().UnixNano())
}

var _ market.MarketDecider = (*SpotDecider)(nil)
