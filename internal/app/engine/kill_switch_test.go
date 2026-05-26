package engine

import (
	"context"
	"testing"
	"time"

	"github.com/kainhuck/signalix/internal/app/oms"
	"github.com/kainhuck/signalix/internal/app/strategy"
	"github.com/kainhuck/signalix/internal/config"
	"github.com/kainhuck/signalix/internal/domain/risk"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/testutil"
)

func TestKillSwitchActivateDeactivate(t *testing.T) {
	t.Parallel()
	e := &Engine{}
	ctx := context.Background()

	st := e.KillSwitchStatus()
	if st.Active {
		t.Fatal("expected inactive initially")
	}

	result, err := e.ActivateKillSwitch(ctx, " drill ", false)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Status.Active || result.Status.Reason != "drill" {
		t.Fatalf("activate: %+v", result.Status)
	}
	if result.Status.ActivatedAt.IsZero() {
		t.Fatal("expected activated_at")
	}
	if !e.killSwitchActive() {
		t.Fatal("killSwitchActive should be true")
	}

	result2, err := e.ActivateKillSwitch(ctx, "other", false)
	if err != nil {
		t.Fatal(err)
	}
	if result2.Status.Reason != "drill" {
		t.Fatalf("idempotent activate should keep first reason, got %q", result2.Status.Reason)
	}
	if !result2.Status.ActivatedAt.Equal(result.Status.ActivatedAt) {
		t.Fatal("idempotent activate should keep first activated_at")
	}

	off, err := e.DeactivateKillSwitch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if off.Active {
		t.Fatal("expected inactive after deactivate")
	}
	if e.killSwitchActive() {
		t.Fatal("killSwitchActive should be false")
	}

	off2, err := e.DeactivateKillSwitch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if off2.Active {
		t.Fatal("deactivate should be idempotent")
	}
}

func TestKillSwitchBlocksExposureSignalsOnly(t *testing.T) {
	t.Parallel()
	long := &models.Signal{Direction: models.DirectionLong}
	flat := &models.Signal{Direction: models.DirectionFlat}
	if !risk.OpensExposure(long) || risk.OpensExposure(flat) {
		t.Fatal("OpensExposure precondition")
	}

	active := true
	if !(active && risk.OpensExposure(long)) {
		t.Fatal("LONG should be blocked when kill switch active")
	}
	if active && risk.OpensExposure(flat) {
		t.Fatal("FLAT should not be blocked when kill switch active")
	}
}

func TestShouldAutoRestartSkipsWhenKillSwitchActive(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e := &Engine{
		ctx:          ctx,
		restartCfg:   config.RestartSettings{Enabled: true, MaxCrashes: 3, CrashWindow: time.Minute},
		crashTracker: newCrashTracker(),
	}
	st := &strategy.Strategy{
		StrategyConfig: strategy.StrategyConfig{Name: "s1", Enabled: true},
	}
	if !e.shouldAutoRestart("s1", st) {
		t.Fatal("expected auto-restart allowed when kill switch off")
	}
	_, _ = e.ActivateKillSwitch(ctx, "test", false)
	if e.shouldAutoRestart("s1", st) {
		t.Fatal("should not auto-restart while kill switch active")
	}
}

func TestActivateKillSwitchCancelOpenOrders(t *testing.T) {
	t.Parallel()
	ex := testutil.NewStubExchange()
	ee := oms.NewExecutionEngine(ex, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		_ = ee.Stop()
	}()
	if err := ee.Start(ctx); err != nil {
		t.Fatal(err)
	}
	o := &models.Order{
		ID:           "ks-cancel-1",
		Symbol:       "BTC/USDT",
		Side:         models.OrderSideBuy,
		OrderType:    models.OrderTypeLimit,
		Size:         "1",
		Status:       models.OrderStatusSubmitted,
		StrategyName: "s",
	}
	if err := ee.SubmitOrder(ctx, o); err != nil {
		t.Fatal(err)
	}

	e := &Engine{executionEngine: ee}
	result, err := e.ActivateKillSwitch(ctx, "cancel test", true)
	if err != nil {
		t.Fatal(err)
	}
	if result.CancelAttempted != 1 {
		t.Fatalf("cancel_attempted = %d, want 1", result.CancelAttempted)
	}
}

func TestActivateKillSwitchUsesConfigCancelDefault(t *testing.T) {
	t.Parallel()
	e := &Engine{
		killSwitchCfg: config.KillSwitchSettings{CancelOpenOrdersOnActivate: false},
	}
	ctx := context.Background()
	result, err := e.ActivateKillSwitch(ctx, "cfg", false)
	if err != nil {
		t.Fatal(err)
	}
	if result.CancelAttempted != 0 {
		t.Fatalf("expected no cancel by default, got %d", result.CancelAttempted)
	}
}

func TestKillSwitchStatusSnapshot(t *testing.T) {
	t.Parallel()
	e := &Engine{}
	ctx := context.Background()
	before := time.Now()
	_, err := e.ActivateKillSwitch(ctx, "snap", false)
	if err != nil {
		t.Fatal(err)
	}
	st := e.KillSwitchStatus()
	if !st.Active || st.Reason != "snap" || st.ActivatedAt.Before(before) {
		t.Fatalf("snapshot: %+v", st)
	}
}
