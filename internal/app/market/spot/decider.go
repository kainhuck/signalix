package spot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kainhuck/signalix/internal/app/decision"
	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	spotex "github.com/kainhuck/signalix/pkg/exchange/spot"
	"github.com/shopspring/decimal"
)

type SpotDeciderConfig struct {
	Projection         *AccountProjection
	Router             *MarketRouter
	PairMeta           map[spotex.Pair]*spotex.PairMeta
	DefaultSizeDivisor int
}

type SpotDecider struct {
	proj               *AccountProjection
	router             *MarketRouter
	pairMeta           map[spotex.Pair]*spotex.PairMeta
	defaultSizeDivisor int
}

func NewSpotDecider(cfg SpotDeciderConfig) market.MarketDecider {
	divisor := cfg.DefaultSizeDivisor
	if divisor <= 0 {
		divisor = 10
	}
	return &SpotDecider{
		proj:               cfg.Projection,
		router:             cfg.Router,
		pairMeta:           clonePairMetaMap(cfg.PairMeta),
		defaultSizeDivisor: divisor,
	}
}

func (d *SpotDecider) Decide(ctx context.Context, strategy string, sig *models.Signal) (*models.Order, error) {
	_ = ctx
	if d == nil || d.proj == nil {
		return nil, fmt.Errorf("spot decider not configured")
	}
	if sig == nil {
		return nil, fmt.Errorf("nil signal")
	}
	if err := sig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid signal: %w", err)
	}
	pair := spotex.CanonicalPair(string(sig.Symbol))
	if pair == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	meta := d.metaForPair(pair)
	if err := validatePairTradable(pair, meta); err != nil {
		return nil, err
	}

	switch sig.Direction {
	case models.DirectionLong:
		return d.decideLong(strategy, sig, pair, meta)
	case models.DirectionShort, models.DirectionFlat:
		return d.decideSell(strategy, sig, pair, meta)
	default:
		return nil, fmt.Errorf("invalid direction: %s", sig.Direction)
	}
}

func (d *SpotDecider) decideLong(strategy string, sig *models.Signal, pair spotex.Pair, meta *spotex.PairMeta) (*models.Order, error) {
	baseBal := d.proj.Balance(pair.BaseCurrency())
	baseTotal := parseDecimal(baseBal.Total)
	if baseTotal.IsPositive() {
		return nil, nil
	}

	quoteBal := d.proj.Balance(pair.QuoteCurrency())
	quoteAvailable := parseDecimal(quoteBal.Available)
	if !quoteAvailable.IsPositive() {
		return nil, nil
	}

	if sig.Price != nil && strings.TrimSpace(*sig.Price) != "" {
		price, err := positiveDecimal(*sig.Price, "limit price")
		if err != nil {
			return nil, err
		}
		baseAmount, err := d.baseAmountForLimitBuy(sig, quoteAvailable, price)
		if err != nil {
			return nil, err
		}
		baseAmount, err = normalizeBaseAmount(baseAmount, meta)
		if err != nil {
			return nil, err
		}
		if baseAmount.IsZero() {
			return nil, nil
		}
		return newSpotOrder(strategy, pair, models.OrderSideBuy, models.OrderTypeLimit, formatBaseAmount(baseAmount, meta), sig.Price), nil
	}

	quoteAmount, err := d.quoteAmountFromSignal(sig, quoteAvailable)
	if err != nil {
		return nil, err
	}
	if err := validateQuoteAmount(quoteAmount, meta); err != nil {
		return nil, err
	}
	if quoteAmount.IsZero() {
		return nil, nil
	}
	return newSpotOrder(strategy, pair, models.OrderSideBuy, models.OrderTypeMarket, quoteAmount.String(), nil), nil
}

func (d *SpotDecider) decideSell(strategy string, sig *models.Signal, pair spotex.Pair, meta *spotex.PairMeta) (*models.Order, error) {
	baseBal := d.proj.Balance(pair.BaseCurrency())
	baseAvailable := parseDecimal(baseBal.Available)
	if !baseAvailable.IsPositive() {
		return nil, nil
	}

	amount := baseAvailable
	if sig.SizingMode != nil && *sig.SizingMode == models.SizingModeCustom {
		if sig.Value == nil {
			return nil, fmt.Errorf("value is required for custom sizing mode")
		}
		custom, err := positiveDecimal(*sig.Value, "custom base amount")
		if err != nil {
			return nil, err
		}
		if custom.GreaterThan(baseAvailable) {
			return nil, fmt.Errorf("custom base amount %s exceeds available %s", custom, baseAvailable)
		}
		amount = custom
	}

	amount, err := normalizeBaseAmount(amount, meta)
	if err != nil {
		return nil, err
	}
	if amount.IsZero() {
		return nil, nil
	}
	orderType := models.OrderTypeMarket
	price := sig.Price
	if price != nil && strings.TrimSpace(*price) != "" {
		if _, err := positiveDecimal(*price, "limit price"); err != nil {
			return nil, err
		}
		orderType = models.OrderTypeLimit
	}
	return newSpotOrder(strategy, pair, models.OrderSideSell, orderType, formatBaseAmount(amount, meta), price), nil
}

func (d *SpotDecider) quoteAmountFromSignal(sig *models.Signal, quoteAvailable decimal.Decimal) (decimal.Decimal, error) {
	if sig.SizingMode == nil {
		return quoteAvailable.Div(decimal.NewFromInt(int64(d.defaultSizeDivisor))), nil
	}
	switch *sig.SizingMode {
	case models.SizingModePercent, models.SizingModeFixed:
		return decision.ComputeUSDTNotionalFromSignal(sig, quoteAvailable, d.defaultSizeDivisor)
	case models.SizingModeCustom:
		if sig.Value == nil {
			return decimal.Zero, fmt.Errorf("value is required for custom sizing mode")
		}
		return positiveDecimal(*sig.Value, "custom quote amount")
	default:
		return decimal.Zero, fmt.Errorf("unknown sizing mode: %s", *sig.SizingMode)
	}
}

func (d *SpotDecider) baseAmountForLimitBuy(sig *models.Signal, quoteAvailable, price decimal.Decimal) (decimal.Decimal, error) {
	if sig.SizingMode != nil && *sig.SizingMode == models.SizingModeCustom {
		if sig.Value == nil {
			return decimal.Zero, fmt.Errorf("value is required for custom sizing mode")
		}
		return positiveDecimal(*sig.Value, "custom base amount")
	}
	quoteAmount, err := d.quoteAmountFromSignal(sig, quoteAvailable)
	if err != nil {
		return decimal.Zero, err
	}
	if quoteAmount.GreaterThan(quoteAvailable) {
		return decimal.Zero, fmt.Errorf("quote amount %s exceeds available %s", quoteAmount, quoteAvailable)
	}
	return quoteAmount.Div(price), nil
}

func (d *SpotDecider) metaForPair(pair spotex.Pair) *spotex.PairMeta {
	if d == nil {
		return nil
	}
	if meta, ok := d.pairMeta[pair.Canonical()]; ok && meta != nil {
		cp := *meta
		return &cp
	}
	if d.proj != nil {
		if meta, ok := d.proj.PairMeta(pair); ok {
			return meta
		}
	}
	return nil
}

func newSpotOrder(strategy string, pair spotex.Pair, side models.OrderSide, typ models.OrderType, size string, price *string) *models.Order {
	now := time.Now()
	return &models.Order{
		ID:           uuid.New().String(),
		Market:       models.MarketSpot,
		Symbol:       perp.Contract(pair.Canonical().String()),
		Side:         side,
		OrderType:    typ,
		Price:        price,
		Size:         size,
		FilledSize:   "0",
		Status:       models.OrderStatusPending,
		CreatedAt:    now,
		UpdatedAt:    now,
		StrategyName: strategy,
	}
}

func validatePairTradable(pair spotex.Pair, meta *spotex.PairMeta) error {
	if meta == nil {
		return nil
	}
	status := strings.ToLower(strings.TrimSpace(meta.TradeStatus))
	if status == "" || status == "tradable" || status == "trading" {
		return nil
	}
	return fmt.Errorf("spot pair %s is not tradable: %s", pair, meta.TradeStatus)
}

func validateQuoteAmount(v decimal.Decimal, meta *spotex.PairMeta) error {
	if !v.IsPositive() {
		return fmt.Errorf("quote amount must be positive")
	}
	if meta == nil || strings.TrimSpace(meta.MinQuoteAmount) == "" {
		return nil
	}
	min := parseDecimal(meta.MinQuoteAmount)
	if min.IsPositive() && v.LessThan(min) {
		return fmt.Errorf("quote amount %s below min %s", v, min)
	}
	return nil
}

func normalizeBaseAmount(v decimal.Decimal, meta *spotex.PairMeta) (decimal.Decimal, error) {
	if !v.IsPositive() {
		return decimal.Zero, nil
	}
	if meta != nil {
		min := parseDecimal(meta.MinBaseAmount)
		if min.IsPositive() && v.LessThan(min) {
			return decimal.Zero, fmt.Errorf("base amount %s below min %s", v, min)
		}
		max := parseDecimal(meta.MaxBaseAmount)
		if max.IsPositive() && v.GreaterThan(max) {
			v = max
		}
		if meta.AmountPrecision >= 0 {
			v = v.Truncate(meta.AmountPrecision)
		}
	}
	return v, nil
}

func formatBaseAmount(v decimal.Decimal, meta *spotex.PairMeta) string {
	if meta != nil && meta.AmountPrecision >= 0 {
		return v.Truncate(meta.AmountPrecision).String()
	}
	return v.String()
}

func positiveDecimal(s, name string) (decimal.Decimal, error) {
	v, err := decimal.NewFromString(strings.TrimSpace(s))
	if err != nil || !v.IsPositive() {
		return decimal.Zero, fmt.Errorf("%s must be positive", name)
	}
	return v, nil
}

func parseDecimal(s string) decimal.Decimal {
	v, err := decimal.NewFromString(strings.TrimSpace(s))
	if err != nil {
		return decimal.Zero
	}
	return v
}

var _ market.MarketDecider = (*SpotDecider)(nil)
