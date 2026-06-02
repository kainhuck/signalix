package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kainhuck/signalix/internal/app/strategy"
	"github.com/kainhuck/signalix/internal/config"
	"github.com/kainhuck/signalix/internal/models"
)

func TestStrategyRuntimeSnapshot_notInCatalog(t *testing.T) {
	t.Parallel()
	e := &Engine{}
	_, err := e.StrategyRuntimeSnapshot("missing")
	if !errors.Is(err, ErrStrategyNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestStrategyRuntimeSnapshot_catalogOnly(t *testing.T) {
	t.Parallel()
	e := &Engine{
		restartCfg: config.DefaultRestartSettings(),
		strategies: make(map[string]*strategy.Strategy),
	}
	e.SetStrategy(&strategy.Strategy{
		StrategyConfig: strategy.StrategyConfig{
			Name:    "alpha",
			Enabled: true,
			Symbols: []string{"BTC/USDT"},
		},
	})
	snap, err := e.StrategyRuntimeSnapshot("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if snap.Running || !snap.LastHeartbeat.IsZero() || snap.CrashCount != 0 {
		t.Fatalf("catalog-only: %+v", snap)
	}
	if !snap.AutoRestartEnabled {
		t.Fatal("expected auto_restart enabled by default")
	}
}

func TestStrategyRuntimeSnapshot_running(t *testing.T) {
	t.Parallel()
	hb := time.Date(2026, 5, 26, 10, 0, 0, 0, time.UTC)
	e := &Engine{
		restartCfg:      config.DefaultRestartSettings(),
		strategies:      make(map[string]*strategy.Strategy),
		strategyProcess: map[string]strategy.StrategyRuntime{},
	}
	e.SetStrategy(&strategy.Strategy{
		StrategyConfig: strategy.StrategyConfig{Name: "s1", Enabled: true},
	})
	sp := &stubStrategyRuntime{name: "s1", running: true, heartbeat: hb, crashes: 2}
	e.strategyProcess["s1"] = sp

	snap, err := e.StrategyRuntimeSnapshot("s1")
	if err != nil {
		t.Fatal(err)
	}
	if !snap.Running || snap.CrashCount != 2 || !snap.LastHeartbeat.Equal(hb) {
		t.Fatalf("snap = %+v", snap)
	}
}

func TestStrategyRuntimeSnapshot_circuitOpen(t *testing.T) {
	t.Parallel()
	window := time.Minute
	ct := newCrashTracker()
	now := time.Now()
	for i := 0; i < 3; i++ {
		ct.record("s1", now.Add(time.Duration(i)*time.Second))
	}
	e := &Engine{
		restartCfg: config.RestartSettings{
			Enabled:     true,
			MaxCrashes:  3,
			CrashWindow: window,
		},
		crashTracker: ct,
		strategies:   make(map[string]*strategy.Strategy),
	}
	e.SetStrategy(&strategy.Strategy{
		StrategyConfig: strategy.StrategyConfig{Name: "s1", Enabled: true},
	})
	snap, err := e.StrategyRuntimeSnapshot("s1")
	if err != nil {
		t.Fatal(err)
	}
	if !snap.CircuitOpen || snap.CrashCountInWindow != 3 {
		t.Fatalf("snap = %+v", snap)
	}
}

func TestListStrategyRuntimeSnapshots(t *testing.T) {
	t.Parallel()
	e := &Engine{
		restartCfg:      config.DefaultRestartSettings(),
		strategies:      make(map[string]*strategy.Strategy),
		strategyProcess: map[string]strategy.StrategyRuntime{},
	}
	e.SetStrategy(&strategy.Strategy{StrategyConfig: strategy.StrategyConfig{Name: "a", Enabled: true}})
	e.SetStrategy(&strategy.Strategy{StrategyConfig: strategy.StrategyConfig{Name: "b", Enabled: false}})
	e.strategyProcess["a"] = &stubStrategyRuntime{name: "a", running: true, heartbeat: time.Now()}

	list := e.ListStrategyRuntimeSnapshots()
	if len(list) != 2 {
		t.Fatalf("len = %d", len(list))
	}
	if list[0].Name != "a" || !list[0].Running {
		t.Fatalf("a: %+v", list[0])
	}
	if list[1].Name != "b" || list[1].Running {
		t.Fatalf("b: %+v", list[1])
	}
}

type stubStrategyRuntime struct {
	name      string
	running   bool
	heartbeat time.Time
	crashes   int
}

func (s *stubStrategyRuntime) Name() string                                     { return s.name }
func (s *stubStrategyRuntime) Context() context.Context                         { return context.Background() }
func (s *stubStrategyRuntime) SendInit(*strategy.Strategy) error                { return nil }
func (s *stubStrategyRuntime) SendTick(*models.Ticker, string) error            { return nil }
func (s *stubStrategyRuntime) SendStop() error                                  { return nil }
func (s *stubStrategyRuntime) ReadMessages(func(strategy.IpcMessage)) error     { return nil }
func (s *stubStrategyRuntime) ReadStderr()                                      {}
func (s *stubStrategyRuntime) Stop() error                                      { return nil }
func (s *stubStrategyRuntime) UpdateHeartbeat()                                 {}
func (s *stubStrategyRuntime) GetLastHeartbeat() time.Time                      { return s.heartbeat }
func (s *stubStrategyRuntime) RecordCrash()                                     {}
func (s *stubStrategyRuntime) PruneCrashes(time.Duration)                       {}
func (s *stubStrategyRuntime) GetCrashCount() int                               { return s.crashes }
func (s *stubStrategyRuntime) IsRunning() bool                                  { return s.running }
func (s *stubStrategyRuntime) SendRPCResponse(string, interface{}, error) error { return nil }
func (s *stubStrategyRuntime) SendKline(*models.Kline, string) error            { return nil }
func (s *stubStrategyRuntime) SendHistory(*models.HistoryPayload) error         { return nil }
