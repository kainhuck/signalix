package risk

import (
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

// EquityTracker 内存权益跟踪：UTC 日切日亏与会话峰值回撤。
type EquityTracker struct {
	mu             sync.RWMutex
	dayStartEquity decimal.Decimal
	dayStartDate   string
	sessionPeak    decimal.Decimal
	lastEquity     decimal.Decimal
}

// NewEquityTracker 创建权益跟踪器。
func NewEquityTracker() *EquityTracker {
	return &EquityTracker{}
}

// OnEquityUpdate 在账户权益变化时调用。
func (t *EquityTracker) OnEquityUpdate(equity decimal.Decimal, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	utcDate := now.UTC().Format("2006-01-02")
	if t.dayStartDate != utcDate {
		t.dayStartDate = utcDate
		t.dayStartEquity = equity
	}
	t.lastEquity = equity
	if equity.GreaterThan(t.sessionPeak) {
		t.sessionPeak = equity
	}
}

// Init 引擎启动时初始化（等价于首次 OnEquityUpdate）。
func (t *EquityTracker) Init(equity decimal.Decimal, now time.Time) {
	t.mu.Lock()
	t.dayStartDate = ""
	t.dayStartEquity = decimal.Zero
	t.sessionPeak = decimal.Zero
	t.lastEquity = decimal.Zero
	t.mu.Unlock()
	t.OnEquityUpdate(equity, now)
}

// DailyLoss 返回当日亏损 USDT（非负）。
func (t *EquityTracker) DailyLoss() decimal.Decimal {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.dayStartEquity.Sign() <= 0 {
		return decimal.Zero
	}
	loss := t.dayStartEquity.Sub(t.lastEquity)
	if loss.Sign() <= 0 {
		return decimal.Zero
	}
	return loss
}

// DrawdownRatio 返回会话峰值回撤比例 0~1。
func (t *EquityTracker) DrawdownRatio() decimal.Decimal {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.sessionPeak.Sign() <= 0 {
		return decimal.Zero
	}
	dd := t.sessionPeak.Sub(t.lastEquity).Div(t.sessionPeak)
	if dd.Sign() < 0 {
		return decimal.Zero
	}
	return dd
}

// SessionPeak 返回会话峰值权益。
func (t *EquityTracker) SessionPeak() decimal.Decimal {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.sessionPeak
}

// ResetSessionPeak 仅测试使用。
func (t *EquityTracker) ResetSessionPeak() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sessionPeak = decimal.Zero
}

// LastEquity 返回最近一次权益采样。
func (t *EquityTracker) LastEquity() decimal.Decimal {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastEquity
}
