package decision

import (
	"context"
	"fmt"
	"strings"

	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"github.com/shopspring/decimal"
)

// TickerLookup 提供引擎侧缓存的 ticker（用于定价）。
type TickerLookup interface {
	GetCachedTicker(symbol perp.Contract) (*perp.TickerSnapshot, bool)
}

func tickerMarkPrice(t *perp.TickerSnapshot) (decimal.Decimal, error) {
	if t == nil {
		return decimal.Zero, fmt.Errorf("nil ticker")
	}
	for _, s := range []string{strings.TrimSpace(t.MarkPrice), strings.TrimSpace(t.Last)} {
		if s == "" {
			continue
		}
		p, err := decimal.NewFromString(s)
		if err != nil {
			return decimal.Zero, fmt.Errorf("parse ticker price %q: %w", s, err)
		}
		if p.IsPositive() {
			return p, nil
		}
	}
	return decimal.Zero, fmt.Errorf("no positive mark/last price for %s", t.Contract)
}

func (de *DecisionEngine) ensureContractMeta(ctx context.Context) error {
	de.metaMu.RLock()
	ready := de.metaLoaded
	de.metaMu.RUnlock()
	if ready {
		return nil
	}
	if de.exchange == nil {
		return fmt.Errorf("exchange not configured for contract meta")
	}
	list, err := de.exchange.ListContractMeta(ctx)
	if err != nil {
		return err
	}
	m := make(map[perp.Contract]*perp.ContractMeta, len(list))
	for _, meta := range list {
		if meta == nil || meta.Contract == "" {
			continue
		}
		cp := *meta
		m[meta.Contract] = &cp
	}
	de.metaMu.Lock()
	de.metaByContract = m
	de.metaLoaded = true
	de.metaMu.Unlock()
	return nil
}

func (de *DecisionEngine) contractMeta(contract perp.Contract) (*perp.ContractMeta, error) {
	de.metaMu.RLock()
	meta, ok := de.metaByContract[contract]
	de.metaMu.RUnlock()
	if !ok || meta == nil {
		return nil, fmt.Errorf("contract meta not found for %s", contract)
	}
	cp := *meta
	return &cp, nil
}

// notionalUSDTToContracts 将 USDT 名义金额换算为 Gate 永续下单张数。
func (de *DecisionEngine) notionalUSDTToContracts(ctx context.Context, contract perp.Contract, notional decimal.Decimal) (decimal.Decimal, error) {
	if notional.IsZero() {
		return decimal.Zero, nil
	}
	if err := de.ensureContractMeta(ctx); err != nil {
		return decimal.Zero, err
	}
	meta, err := de.contractMeta(contract)
	if err != nil {
		return decimal.Zero, err
	}
	mult, err := decimal.NewFromString(strings.TrimSpace(meta.QuantoMultiplier))
	if err != nil || !mult.IsPositive() {
		return decimal.Zero, fmt.Errorf("invalid quanto_multiplier for %s: %q", contract, meta.QuantoMultiplier)
	}
	if de.tickerLookup == nil {
		return decimal.Zero, fmt.Errorf("ticker lookup not configured")
	}
	tick, ok := de.tickerLookup.GetCachedTicker(contract)
	if !ok || tick == nil {
		return decimal.Zero, fmt.Errorf("no cached ticker for %s (wait for futures.tickers)", contract)
	}
	price, err := tickerMarkPrice(tick)
	if err != nil {
		return decimal.Zero, err
	}
	denom := price.Mul(mult)
	if !denom.IsPositive() {
		return decimal.Zero, fmt.Errorf("invalid price×multiplier for %s", contract)
	}
	raw := notional.Div(denom)
	if !meta.EnableDecimalSize {
		raw = raw.Floor()
	}
	minSz := decimal.Zero
	if s := strings.TrimSpace(meta.OrderSizeMin); s != "" {
		minSz, _ = decimal.NewFromString(s)
	}
	if minSz.IsPositive() && raw.LessThan(minSz) {
		if notional.IsPositive() {
			raw = minSz
		}
	}
	if maxStr := strings.TrimSpace(meta.OrderSizeMax); maxStr != "" {
		if maxSz, err := decimal.NewFromString(maxStr); err == nil && maxSz.IsPositive() && raw.GreaterThan(maxSz) {
			raw = maxSz
		}
	}
	if !raw.IsPositive() {
		return decimal.Zero, fmt.Errorf("computed contract size is zero for %s (notional=%s)", contract, notional.String())
	}
	return raw, nil
}

func formatOrderSize(sz decimal.Decimal, meta *perp.ContractMeta) string {
	if meta != nil && meta.EnableDecimalSize {
		return sz.String()
	}
	return sz.Truncate(0).String()
}
