package grpc

import (
	"context"
	"testing"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/kainhuck/signalix/internal/app/engine"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetTicker_engineNotRunning(t *testing.T) {
	t.Parallel()
	svc := NewEngineService(&engine.Engine{})
	_, err := svc.GetTicker(context.Background(), &enginev1.GetTickerRequest{Symbol: "BTC/USDT"})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("code = %v", status.Code(err))
	}
}

func TestGetKlines_engineNotRunning(t *testing.T) {
	t.Parallel()
	svc := NewEngineService(&engine.Engine{})
	_, err := svc.GetKlines(context.Background(), &enginev1.GetKlinesRequest{Symbol: "BTC/USDT", Interval: "5m"})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("code = %v", status.Code(err))
	}
}

func TestListTickers_engineNotRunning(t *testing.T) {
	t.Parallel()
	svc := NewEngineService(&engine.Engine{})
	_, err := svc.ListTickers(context.Background(), &enginev1.ListTickersRequest{})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("code = %v", status.Code(err))
	}
}
