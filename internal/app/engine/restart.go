package engine

import (
	"math"
	"sync"
	"time"

	"github.com/kainhuck/signalix/internal/app/strategy"
	"github.com/kainhuck/signalix/internal/config"
	"github.com/kainhuck/signalix/pkg/logger"
)

const maxConsecutiveRestartFailures = 3

type crashTracker struct {
	mu    sync.Mutex
	times map[string][]time.Time
}

func newCrashTracker() *crashTracker {
	return &crashTracker{times: make(map[string][]time.Time)}
}

func (ct *crashTracker) record(name string, at time.Time) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.times[name] = append(ct.times[name], at)
}

func (ct *crashTracker) pruneAndCount(name string, window time.Duration, now time.Time) int {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.times[name] = pruneCrashTimes(ct.times[name], window, now)
	return len(ct.times[name])
}

func (ct *crashTracker) countInWindow(name string, window time.Duration, now time.Time) int {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	return len(pruneCrashTimes(ct.times[name], window, now))
}

func pruneCrashTimes(times []time.Time, window time.Duration, now time.Time) []time.Time {
	if window <= 0 || len(times) == 0 {
		return times
	}
	cutoff := now.Add(-window)
	out := times[:0]
	for _, t := range times {
		if !t.Before(cutoff) {
			out = append(out, t)
		}
	}
	return out
}

func computeBackoff(cfg config.RestartSettings, attempt int) time.Duration {
	if cfg.InitialBackoff <= 0 {
		return 0
	}
	mult := cfg.BackoffMultiplier
	if mult <= 0 {
		mult = 2
	}
	secs := cfg.InitialBackoff.Seconds()
	for i := 0; i < attempt; i++ {
		secs *= float64(mult)
	}
	if cfg.MaxBackoff > 0 {
		secs = math.Min(secs, cfg.MaxBackoff.Seconds())
	}
	return time.Duration(secs * float64(time.Second))
}

func (e *Engine) resetRestartAttempt(name string) {
	e.restartMu.Lock()
	delete(e.restartAttempt, name)
	e.restartMu.Unlock()
}

func (e *Engine) getRestartAttempt(name string) int {
	e.restartMu.Lock()
	defer e.restartMu.Unlock()
	return e.restartAttempt[name]
}

func (e *Engine) bumpRestartAttempt(name string) {
	e.restartMu.Lock()
	e.restartAttempt[name]++
	e.restartMu.Unlock()
}

func (e *Engine) setPendingRestart(name string, until time.Time) {
	e.pendingRestartMu.Lock()
	e.pendingRestart[name] = until
	e.pendingRestartMu.Unlock()
}

func (e *Engine) clearPendingRestart(name string) {
	e.pendingRestartMu.Lock()
	delete(e.pendingRestart, name)
	e.pendingRestartMu.Unlock()
}

func (e *Engine) pendingRestartSeconds(name string) float64 {
	e.pendingRestartMu.Lock()
	until, ok := e.pendingRestart[name]
	e.pendingRestartMu.Unlock()
	if !ok {
		return 0
	}
	rem := time.Until(until).Seconds()
	if rem < 0 {
		return 0
	}
	return rem
}

func (e *Engine) recordStrategyCrash(name string) int {
	now := time.Now()
	e.crashTracker.record(name, now)
	return e.crashTracker.pruneAndCount(name, e.restartCfg.CrashWindow, now)
}

func (e *Engine) crashCountInWindow(name string) int {
	return e.crashTracker.countInWindow(name, e.restartCfg.CrashWindow, time.Now())
}

func (e *Engine) circuitOpen(name string) bool {
	return e.crashCountInWindow(name) >= e.restartCfg.MaxCrashes
}

func (e *Engine) shouldAutoRestart(name string, st *strategy.Strategy) bool {
	if e.ctx.Err() != nil {
		return false
	}
	if !e.restartCfg.Enabled {
		return false
	}
	if e.killSwitchActive() {
		return false
	}
	if st == nil || !st.Enabled {
		return false
	}
	if e.circuitOpen(name) {
		return false
	}
	return true
}

func (e *Engine) scheduleRestart(name string, st *strategy.Strategy) {
	e.restartWg.Add(1)
	go func() {
		defer e.restartWg.Done()

		attempt := e.getRestartAttempt(name)
		backoff := computeBackoff(e.restartCfg, attempt)

		logger.WarnContext(e.ctx, "Strategy restart scheduled",
			logger.String("strategy", name),
			logger.Duration("backoff", backoff),
			logger.Int("attempt", attempt))

		wakeAt := time.Now().Add(backoff)
		e.setPendingRestart(name, wakeAt)
		defer e.clearPendingRestart(name)

		timer := time.NewTimer(backoff)
		defer timer.Stop()

		select {
		case <-timer.C:
		case <-e.ctx.Done():
			return
		}

		if !e.shouldAutoRestart(name, st) {
			return
		}

		failures := 0
		for failures < maxConsecutiveRestartFailures {
			if e.ctx.Err() != nil {
				return
			}
			if !e.shouldAutoRestart(name, st) {
				return
			}

			err := e.launchStrategy(st)
			if err == nil {
				e.bumpRestartAttempt(name)
				logger.InfoContext(e.ctx, "Strategy auto-restart succeeded",
					logger.String("strategy", name),
					logger.Int("attempt", e.getRestartAttempt(name)),
				)
				return
			}

			failures++
			count := e.recordStrategyCrash(name)
			logger.ErrorContext(e.ctx, "Strategy auto-restart failed",
				logger.String("strategy", name),
				logger.Any("error", err),
				logger.Int("failures", failures),
				logger.Int("crash_count", count))

			if e.circuitOpen(name) {
				logger.ErrorContext(e.ctx, "Strategy restart circuit open after failures",
					logger.String("strategy", name),
					logger.Int("crash_count", count),
					logger.Int("max_crashes", e.restartCfg.MaxCrashes))
				return
			}
		}

		logger.ErrorContext(e.ctx, "Strategy restart abandoned after consecutive failures",
			logger.String("strategy", name),
			logger.Int("max_failures", maxConsecutiveRestartFailures))
	}()
}
