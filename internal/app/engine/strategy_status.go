package engine

import (
	"errors"
	"time"

	"github.com/kainhuck/signalix/internal/app/strategy"
)

// ErrStrategyNotFound 表示策略不在 catalog 中。
var ErrStrategyNotFound = errors.New("strategy not found")

// StrategyRuntimeSnapshot 策略 catalog + 运行时只读视图。
type StrategyRuntimeSnapshot struct {
	Name               string
	Enabled            bool
	Symbols            []string
	Running            bool
	LastHeartbeat      time.Time
	CrashCount         int
	CrashCountInWindow int
	AutoRestartEnabled bool
	RestartBackoffSec  float64
	CircuitOpen        bool
}

// StrategyRuntimeSnapshot 按名称构建；不在 catalog 返回 ErrStrategyNotFound。
func (e *Engine) StrategyRuntimeSnapshot(name string) (StrategyRuntimeSnapshot, error) {
	st, ok := e.GetStrategy(name)
	if !ok || st == nil {
		return StrategyRuntimeSnapshot{}, ErrStrategyNotFound
	}
	return e.buildRuntimeSnapshot(st), nil
}

// ListStrategyRuntimeSnapshots 返回 catalog 全量及各条目运行时字段。
func (e *Engine) ListStrategyRuntimeSnapshots() []StrategyRuntimeSnapshot {
	list := e.ListStrategyCatalog()
	out := make([]StrategyRuntimeSnapshot, 0, len(list))
	for _, st := range list {
		if st == nil {
			continue
		}
		out = append(out, e.buildRuntimeSnapshot(st))
	}
	return out
}

func (e *Engine) buildRuntimeSnapshot(st *strategy.Strategy) StrategyRuntimeSnapshot {
	name := st.Name
	snap := StrategyRuntimeSnapshot{
		Name:               name,
		Enabled:            st.Enabled,
		Symbols:            append([]string(nil), st.Symbols...),
		AutoRestartEnabled: e.restartCfg.Enabled,
		CrashCountInWindow: e.crashCountInWindow(name),
		RestartBackoffSec:  e.pendingRestartSeconds(name),
		CircuitOpen:        e.circuitOpen(name),
	}
	e.processMu.RLock()
	if sp, ok := e.strategyProcess[name]; ok && sp != nil {
		snap.Running = sp.IsRunning()
		if snap.Running {
			snap.LastHeartbeat = sp.GetLastHeartbeat()
		}
		snap.CrashCount = sp.GetCrashCount()
	}
	e.processMu.RUnlock()
	return snap
}

func strategyRuntimeSnapshotToMap(s StrategyRuntimeSnapshot) map[string]interface{} {
	m := map[string]interface{}{
		"name":                  s.Name,
		"running":               s.Running,
		"crash_count":           s.CrashCount,
		"crash_count_in_window": s.CrashCountInWindow,
		"auto_restart_enabled":  s.AutoRestartEnabled,
		"restart_backoff_sec":   s.RestartBackoffSec,
		"circuit_open":          s.CircuitOpen,
	}
	if !s.LastHeartbeat.IsZero() {
		m["last_heartbeat"] = s.LastHeartbeat
	}
	return m
}
