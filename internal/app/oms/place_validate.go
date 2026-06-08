package oms

import (
	"fmt"
	"strings"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"github.com/shopspring/decimal"
)

func (e *ExecutionEngine) validateOrderForPlace(order *models.Order) error {
	if order == nil {
		return fmt.Errorf("nil order")
	}
	if e == nil || e.metaLookup == nil {
		return nil
	}
	meta, err := 	e.metaLookup.ContractMeta(perp.Contract(order.Symbol))
	if err != nil {
		return err
	}
	if err := validateOrderSize(meta, order.Size); err != nil {
		return err
	}
	if order.OrderType == models.OrderTypeLimit && order.Price != nil && strings.TrimSpace(*order.Price) != "" {
		if err := validateLimitPrice(meta, *order.Price); err != nil {
			return err
		}
	}
	return nil
}

func validateOrderSize(meta *perp.ContractMeta, sizeStr string) error {
	if meta == nil {
		return fmt.Errorf("nil contract meta")
	}
	size, err := decimal.NewFromString(strings.TrimSpace(sizeStr))
	if err != nil || !size.IsPositive() {
		return fmt.Errorf("invalid order size %q", sizeStr)
	}
	if !meta.EnableDecimalSize && !size.Equal(size.Truncate(0)) {
		return fmt.Errorf("size must be integer contracts for %s", meta.Contract)
	}
	if min, ok := parseOptionalPositiveDecimal(meta.OrderSizeMin); ok && size.LessThan(min) {
		return fmt.Errorf("size %s below minimum %s for %s", size.String(), min.String(), meta.Contract)
	}
	if max, ok := parseOptionalPositiveDecimal(meta.OrderSizeMax); ok && size.GreaterThan(max) {
		return fmt.Errorf("size %s above maximum %s for %s", size.String(), max.String(), meta.Contract)
	}
	if step, ok := parseOptionalPositiveDecimal(meta.OrderSizeStep); ok && !size.Mod(step).IsZero() {
		return fmt.Errorf("size %s not aligned to step %s for %s", size.String(), step.String(), meta.Contract)
	}
	return nil
}

func validateLimitPrice(meta *perp.ContractMeta, priceStr string) error {
	if meta == nil {
		return fmt.Errorf("nil contract meta")
	}
	price, err := decimal.NewFromString(strings.TrimSpace(priceStr))
	if err != nil || !price.IsPositive() {
		return fmt.Errorf("invalid limit price %q", priceStr)
	}
	if step, ok := parseOptionalPositiveDecimal(meta.OrderPriceStep); ok && !price.Mod(step).IsZero() {
		return fmt.Errorf("price %s not aligned to tick %s for %s", price.String(), step.String(), meta.Contract)
	}
	return nil
}

func parseOptionalPositiveDecimal(s string) (decimal.Decimal, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return decimal.Zero, false
	}
	d, err := decimal.NewFromString(s)
	if err != nil || !d.IsPositive() {
		return decimal.Zero, false
	}
	return d, true
}
