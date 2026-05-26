package engine

import (
	"context"
	"strings"
	"time"

	"github.com/kainhuck/signalix/pkg/logger"
)

// KillSwitchStatus Kill Switch 只读快照。
type KillSwitchStatus struct {
	Active      bool
	ActivatedAt time.Time
	Reason      string
}

// KillSwitchActivateResult 激活结果（含撤单统计）。
type KillSwitchActivateResult struct {
	Status          KillSwitchStatus
	CancelAttempted int
	CancelFailed    int
}

type killSwitchState struct {
	active      bool
	activatedAt time.Time
	reason      string
}

func (e *Engine) killSwitchActive() bool {
	e.killSwitchMu.RLock()
	defer e.killSwitchMu.RUnlock()
	return e.killSwitch.active
}

// KillSwitchStatus 返回当前 Kill Switch 状态快照。
func (e *Engine) KillSwitchStatus() KillSwitchStatus {
	e.killSwitchMu.RLock()
	defer e.killSwitchMu.RUnlock()
	return e.killSwitchSnapshotLocked()
}

func (e *Engine) killSwitchSnapshotLocked() KillSwitchStatus {
	st := KillSwitchStatus{Reason: e.killSwitch.reason}
	if e.killSwitch.active {
		st.Active = true
		st.ActivatedAt = e.killSwitch.activatedAt
	}
	return st
}

// ActivateKillSwitch 激活 Kill Switch；重复激活幂等（保留首次 reason/时间）。
func (e *Engine) ActivateKillSwitch(ctx context.Context, reason string, cancelOpenOrders bool) (KillSwitchActivateResult, error) {
	reason = strings.TrimSpace(reason)

	e.killSwitchMu.Lock()
	firstActivation := !e.killSwitch.active
	if firstActivation {
		e.killSwitch.active = true
		e.killSwitch.activatedAt = time.Now()
		e.killSwitch.reason = reason
	}
	status := e.killSwitchSnapshotLocked()
	e.killSwitchMu.Unlock()

	result := KillSwitchActivateResult{Status: status}

	shouldCancel := cancelOpenOrders
	if !cancelOpenOrders {
		shouldCancel = e.killSwitchCfg.CancelOpenOrdersOnActivate
	}
	if firstActivation && shouldCancel {
		attempted, failed := e.cancelAllOpenOrders(ctx)
		result.CancelAttempted = attempted
		result.CancelFailed = failed
		logger.InfoContext(ctx, "kill switch activated with cancel open orders",
			logger.Int("cancel_attempted", attempted),
			logger.Int("cancel_failed", failed),
			logger.String("reason", status.Reason))
	} else if firstActivation {
		logger.InfoContext(ctx, "kill switch activated", logger.String("reason", status.Reason))
	}

	return result, nil
}

// DeactivateKillSwitch 解除 Kill Switch（幂等）。
func (e *Engine) DeactivateKillSwitch(ctx context.Context) (KillSwitchStatus, error) {
	e.killSwitchMu.Lock()
	wasActive := e.killSwitch.active
	e.killSwitch.active = false
	e.killSwitch.activatedAt = time.Time{}
	e.killSwitch.reason = ""
	status := e.killSwitchSnapshotLocked()
	e.killSwitchMu.Unlock()

	if wasActive {
		logger.InfoContext(ctx, "kill switch deactivated")
	}
	return status, nil
}

func (e *Engine) cancelAllOpenOrders(ctx context.Context) (attempted, failed int) {
	if e.executionEngine == nil {
		return 0, 0
	}
	orders := e.ListOpenOrdersSnapshot(500)
	for _, o := range orders {
		if o == nil || o.ID == "" {
			continue
		}
		attempted++
		if err := e.CancelOrderViaOMS(ctx, o.ID); err != nil {
			failed++
			logger.ErrorContext(ctx, "kill switch cancel order failed",
				logger.String("order_id", o.ID),
				logger.Any("error", err))
		}
	}
	return attempted, failed
}
