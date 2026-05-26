package grpc

import (
	"context"
	"testing"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/kainhuck/signalix/internal/app/engine"
	"github.com/kainhuck/signalix/internal/app/strategy"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetStrategyStatus_engineNotRunning(t *testing.T) {
	t.Parallel()
	svc := NewEngineService(&engine.Engine{})
	_, err := svc.GetStrategyStatus(context.Background(), &enginev1.GetStrategyStatusRequest{Name: "x"})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("code = %v", status.Code(err))
	}
}

func TestListStrategies_engineNotRunning(t *testing.T) {
	t.Parallel()
	e := &engine.Engine{}
	e.SetAllStrategy(map[string]*strategy.Strategy{
		"alpha": {
			StrategyConfig: strategy.StrategyConfig{
				Name:    "alpha",
				Enabled: true,
				Symbols: []perp.Contract{"BTC/USDT"},
			},
		},
	})
	svc := NewEngineService(e)
	reply, err := svc.ListStrategies(context.Background(), &enginev1.ListStrategiesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(reply.GetStrategies()) != 1 || reply.GetStrategies()[0].GetName() != "alpha" {
		t.Fatalf("reply = %+v", reply.GetStrategies())
	}
	if reply.GetStrategies()[0].GetRunning() {
		t.Fatal("expected not running")
	}
}
