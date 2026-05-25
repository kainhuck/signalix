package decision

import (
	"context"
	"fmt"
	"strings"

	"github.com/kainhuck/signalix/internal/domain/risk"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"github.com/shopspring/decimal"
)

// NotionalForContracts 将张数换算为 USDT 名义价值。
func (de *DecisionEngine) NotionalForContracts(ctx context.Context, contract perp.Contract, size decimal.Decimal, positionMarkPrice string) (decimal.Decimal, error) {
	if size.IsZero() {
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
		return decimal.Zero, fmt.Errorf("invalid quanto_multiplier for %s", contract)
	}
	mark, err := de.resolveMarkPrice(contract, positionMarkPrice)
	if err != nil {
		return decimal.Zero, err
	}
	return risk.ContractNotionalUSDT(size, mark, mult), nil
}

// ContractsForNotional 将 USDT 名义价值换算为张数。
func (de *DecisionEngine) ContractsForNotional(ctx context.Context, contract perp.Contract, notional decimal.Decimal) (decimal.Decimal, error) {
	return de.notionalUSDTToContracts(ctx, contract, notional)
}

func (de *DecisionEngine) resolveMarkPrice(contract perp.Contract, positionMarkPrice string) (decimal.Decimal, error) {
	if s := strings.TrimSpace(positionMarkPrice); s != "" {
		p, err := decimal.NewFromString(s)
		if err == nil && p.IsPositive() {
			return p, nil
		}
	}
	if de.tickerLookup == nil {
		return decimal.Zero, fmt.Errorf("ticker lookup not configured")
	}
	tick, ok := de.tickerLookup.GetCachedTicker(contract)
	if !ok || tick == nil {
		return decimal.Zero, fmt.Errorf("no cached ticker for %s", contract)
	}
	return tickerMarkPrice(tick)
}
