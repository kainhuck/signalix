package projection

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kainhuck/signalix/internal/ports"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"github.com/shopspring/decimal"
)

// DefaultProjectionRefreshInterval REST 校准周期（一期常量）。
const DefaultProjectionRefreshInterval = 10 * time.Second

// ErrProjectionNotReady 表示尚未完成至少一次成功的 REST 刷新，快照不可用。
var ErrProjectionNotReady = errors.New("account projection not ready")

// AccountProjection 账户余额与持仓的内存投影：定时 REST 全量校准 + OMS 同步 OnUserEvent。
type AccountProjection struct {
	exchange        ports.Exchange
	mu              sync.RWMutex
	balance         *perp.BalanceView
	positions       map[perp.Contract]*perp.PositionSnapshot
	revision        uint64
	ready           bool
	refreshInterval time.Duration
	equityHook      EquityHook
	refreshHook     RefreshHook

	runWg    sync.WaitGroup
	stopOnce sync.Once
}

// NewAccountProjection 创建账户投影；exchange 用于 Balance/Positions。
func NewAccountProjection(exchange ports.Exchange, opts ...Option) *AccountProjection {
	p := &AccountProjection{
		exchange:        exchange,
		positions:       make(map[perp.Contract]*perp.PositionSnapshot),
		refreshInterval: DefaultProjectionRefreshInterval,
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Start 冷启动一次 refresh，再启动定时校准；ctx 取消时后台循环退出。
func (p *AccountProjection) Start(ctx context.Context) error {
	if err := p.refresh(ctx); err != nil {
		return err
	}

	p.runWg.Add(1)
	go func() {
		defer p.runWg.Done()
		t := time.NewTicker(p.refreshInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				_ = p.refresh(ctx)
			}
		}
	}()
	return nil
}

// SetEquityHook 在 Start 之前注册权益钩子（供引擎注入 EquityTracker）。
func (p *AccountProjection) SetEquityHook(h EquityHook) {
	if p == nil || h == nil {
		return
	}
	p.mu.Lock()
	p.equityHook = h
	p.mu.Unlock()
}

// Stop 等待后台 goroutine 结束；幂等。
func (p *AccountProjection) Stop() {
	p.stopOnce.Do(func() {
		p.runWg.Wait()
	})
}

// AccountEquity 返回当前 USDT 总权益。
func (p *AccountProjection) AccountEquity() (decimal.Decimal, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.ready || p.balance == nil {
		return decimal.Zero, ErrProjectionNotReady
	}
	total, err := decimal.NewFromString(strings.TrimSpace(p.balance.Total))
	if err != nil {
		return decimal.Zero, err
	}
	return total, nil
}

// PositionForContract 返回指定合约持仓；无仓返回 (nil, nil)。
func (p *AccountProjection) PositionForContract(contract perp.Contract) (*perp.PositionSnapshot, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.ready {
		return nil, ErrProjectionNotReady
	}
	pv, ok := p.positions[contract]
	if !ok || pv == nil {
		return nil, nil
	}
	cp := *pv
	return &cp, nil
}

// AllPositions 返回当前持仓快照副本。
func (p *AccountProjection) AllPositions() ([]*perp.PositionSnapshot, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.ready {
		return nil, ErrProjectionNotReady
	}
	out := make([]*perp.PositionSnapshot, 0, len(p.positions))
	for _, pv := range p.positions {
		if pv == nil {
			continue
		}
		cp := *pv
		out = append(out, &cp)
	}
	return out, nil
}

// SetRefreshHook 运行时设置 REST refresh 成功钩子。
func (p *AccountProjection) SetRefreshHook(h RefreshHook) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.refreshHook = h
	p.mu.Unlock()
}

// IsReady 投影是否已完成至少一次成功 refresh。
func (p *AccountProjection) IsReady() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.ready
}

// Snapshot 返回当前合约下的余额与持仓快照（无仓时 position 为 nil）及 revision；仅 RLock。
func (p *AccountProjection) Snapshot(contract perp.Contract) (balance *perp.BalanceView, position *perp.PositionSnapshot, revision uint64, err error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.ready || p.balance == nil {
		return nil, nil, 0, ErrProjectionNotReady
	}
	bal := *p.balance
	if pv, ok := p.positions[contract]; ok && pv != nil {
		pos := *pv
		position = &pos
	}
	return &bal, position, p.revision, nil
}

// OpenPositionCount 返回当前有持仓的合约条数（len(positions)）；未就绪返回错误。
func (p *AccountProjection) OpenPositionCount() (int, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.ready {
		return 0, ErrProjectionNotReady
	}
	return len(p.positions), nil
}

// OnUserEvent 由 OMS 在 dispatchUserEvent 之前同步调用；在 ready 后合并余额/持仓 WS 增量。
func (p *AccountProjection) OnUserEvent(_ context.Context, ev *perp.UserEvent) {
	if ev == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.ready {
		return
	}
	switch ev.Kind {
	case perp.UserPositionUpdate:
		p.applyPositionUpdate(ev)
	case perp.UserBalanceUpdate:
		p.applyBalanceUpdate(ev)
	}
}

func (p *AccountProjection) applyPositionUpdate(ev *perp.UserEvent) {
	pv, ok := ev.Position()
	if !ok || pv == nil || pv.Contract == "" {
		return
	}
	if positionSizeIsZero(pv.Size) {
		delete(p.positions, pv.Contract)
		p.revision++
		return
	}
	c := *pv
	p.positions[pv.Contract] = &c
	p.revision++
}

func (p *AccountProjection) applyBalanceUpdate(ev *perp.UserEvent) {
	snap, ok := ev.Balance()
	if !ok || snap == nil || p.balance == nil {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(snap.Currency), strings.TrimSpace(p.balance.Currency)) {
		return
	}
	total := strings.TrimSpace(snap.Balance)
	if total == "" {
		return
	}
	p.balance.Total = total
	if snap.UpdatedAt.IsZero() {
		p.balance.UpdatedAt = time.Now()
	} else {
		p.balance.UpdatedAt = snap.UpdatedAt
	}
	p.revision++
	p.notifyEquityLocked(time.Now())
}

func (p *AccountProjection) notifyEquityLocked(now time.Time) {
	if p.equityHook == nil || p.balance == nil {
		return
	}
	eq, err := decimal.NewFromString(strings.TrimSpace(p.balance.Total))
	if err != nil {
		return
	}
	p.equityHook(eq, now)
}

func positionSizeIsZero(size string) bool {
	f, err := strconv.ParseFloat(strings.TrimSpace(size), 64)
	if err != nil {
		return strings.TrimSpace(size) == "" || strings.TrimSpace(size) == "0"
	}
	return f == 0
}

func (p *AccountProjection) refresh(ctx context.Context) error {
	bal, err := p.exchange.Balance(ctx)
	if err != nil {
		return err
	}
	list, err := p.exchange.Positions(ctx)
	if err != nil {
		return err
	}

	m := make(map[perp.Contract]*perp.PositionSnapshot, len(list))
	for _, pv := range list {
		if pv == nil {
			continue
		}
		c := *pv
		m[pv.Contract] = &c
	}
	bc := *bal

	var hook RefreshHook
	var hookBalance perp.BalanceView
	var hookPositions []*perp.PositionSnapshot
	var hookRevision uint64
	var hookAt time.Time

	p.mu.Lock()
	p.balance = &bc
	p.positions = m
	p.revision++
	p.ready = true
	now := time.Now()
	p.notifyEquityLocked(now)
	if p.refreshHook != nil {
		hookBalance = bc
		hookPositions = snapshotPositionsCopy(m)
		hookRevision = p.revision
		hookAt = now.UTC()
		hook = p.refreshHook
	}
	p.mu.Unlock()

	if hook != nil {
		hook(&hookBalance, hookPositions, hookRevision, hookAt)
	}
	return nil
}

func snapshotPositionsCopy(m map[perp.Contract]*perp.PositionSnapshot) []*perp.PositionSnapshot {
	if len(m) == 0 {
		return []*perp.PositionSnapshot{}
	}
	keys := make([]string, 0, len(m))
	for c := range m {
		keys = append(keys, string(c))
	}
	sort.Strings(keys)
	out := make([]*perp.PositionSnapshot, 0, len(keys))
	for _, k := range keys {
		pv := m[perp.Contract(k)]
		if pv == nil {
			continue
		}
		c := *pv
		out = append(out, &c)
	}
	return out
}
