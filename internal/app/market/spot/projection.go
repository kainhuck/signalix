package spot

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
	spotex "github.com/kainhuck/signalix/pkg/exchange/spot"
)

type AccountProjection struct {
	exchange ports.SpotExchange
	pairMeta map[spotex.Pair]*spotex.PairMeta

	mu       sync.RWMutex
	balances map[string]*spotex.BalanceView
	ready    bool
}

func NewAccountProjection(exchange ports.SpotExchange, pairMeta map[spotex.Pair]*spotex.PairMeta) *AccountProjection {
	return &AccountProjection{
		exchange: exchange,
		pairMeta: clonePairMetaMap(pairMeta),
		balances: make(map[string]*spotex.BalanceView),
	}
}

func (p *AccountProjection) Start(ctx context.Context) error {
	if p == nil || p.exchange == nil {
		return fmt.Errorf("spot account projection not configured")
	}
	balances, err := p.exchange.Balances(ctx)
	if err != nil {
		return fmt.Errorf("spot balances load: %w", err)
	}
	p.mu.Lock()
	for _, bal := range balances {
		p.setBalanceLocked(balanceViewFromSpotExchange(bal))
	}
	p.ready = true
	p.mu.Unlock()
	return nil
}

func (p *AccountProjection) Stop() {
	// User stream is consumed by SpotExecutor so order and balance events are not split
	// between competing readers.
}

func (p *AccountProjection) OnUserEvent(ev *spotex.UserEvent) {
	if ev == nil {
		return
	}
	if bal, ok := ev.Balance(); ok {
		p.mu.Lock()
		p.setBalanceLocked(balanceViewFromBalanceUpdate(bal))
		p.mu.Unlock()
	}
}

func (p *AccountProjection) Balance(currency string) *spotex.BalanceView {
	currency = normalizeCurrency(currency)
	p.mu.RLock()
	defer p.mu.RUnlock()
	if bal, ok := p.balances[currency]; ok && bal != nil {
		cp := *bal
		return &cp
	}
	return &spotex.BalanceView{Currency: currency}
}

func (p *AccountProjection) Position(pair spotex.Pair) *models.PositionView {
	pair = pair.Canonical()
	if pair == "" {
		return nil
	}
	bal := p.Balance(pair.BaseCurrency())
	if bal == nil || bal.Total == "" || bal.Total == "0" {
		return nil
	}
	return positionViewFromSpot(pair, bal)
}

func (p *AccountProjection) ListPositions() []*models.PositionView {
	p.mu.RLock()
	defer p.mu.RUnlock()

	out := make([]*models.PositionView, 0)
	seenBase := make(map[string]bool)
	for pair := range p.pairMeta {
		base := pair.BaseCurrency()
		if base == "" || seenBase[base] {
			continue
		}
		bal := p.balances[base]
		if bal == nil || bal.Total == "" || bal.Total == "0" {
			continue
		}
		seenBase[base] = true
		out = append(out, positionViewFromSpot(pair, bal))
	}
	return out
}

func (p *AccountProjection) IsReady() bool {
	if p == nil {
		return false
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.ready
}

func (p *AccountProjection) PairMeta(pair spotex.Pair) (*spotex.PairMeta, bool) {
	if p == nil {
		return nil, false
	}
	meta, ok := p.pairMeta[pair.Canonical()]
	if !ok || meta == nil {
		return nil, false
	}
	cp := *meta
	return &cp, true
}

func (p *AccountProjection) Balances() map[string]*spotex.BalanceView {
	if p == nil {
		return nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make(map[string]*spotex.BalanceView, len(p.balances))
	for ccy, bal := range p.balances {
		if bal == nil {
			continue
		}
		cp := *bal
		out[ccy] = &cp
	}
	return out
}

func (p *AccountProjection) setBalanceLocked(bal *spotex.BalanceView) {
	if bal == nil {
		return
	}
	bal.Currency = normalizeCurrency(bal.Currency)
	if bal.Currency == "" {
		return
	}
	cp := *bal
	p.balances[bal.Currency] = &cp
}

func balanceViewFromSpotExchange(b *spotex.BalanceView) *spotex.BalanceView {
	if b == nil {
		return nil
	}
	cp := *b
	cp.Currency = normalizeCurrency(cp.Currency)
	return &cp
}

func balanceViewFromBalanceUpdate(b *spotex.BalanceUpdateSnapshot) *spotex.BalanceView {
	if b == nil {
		return nil
	}
	return &spotex.BalanceView{
		Currency:  normalizeCurrency(b.Currency),
		Total:     b.Total,
		Available: b.Available,
		Frozen:    b.Frozen,
		UpdatedAt: b.UpdatedAt,
	}
}

func balanceViewToModel(b *spotex.BalanceView) *models.BalanceView {
	if b == nil {
		return nil
	}
	return &models.BalanceView{
		Currency:  normalizeCurrency(b.Currency),
		Total:     b.Total,
		Available: b.Available,
		Frozen:    b.Frozen,
		UpdatedAt: b.UpdatedAt,
	}
}

func positionViewFromSpot(pair spotex.Pair, bal *spotex.BalanceView) *models.PositionView {
	if bal == nil {
		return nil
	}
	return &models.PositionView{
		Market:    models.MarketSpot,
		Symbol:    pair.Canonical().String(),
		Side:      "long",
		Size:      bal.Total,
		UpdatedAt: bal.UpdatedAt,
	}
}

func normalizeCurrency(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}
