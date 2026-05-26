package grpc

import (
	"context"
	"testing"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/kainhuck/signalix/internal/app/engine"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetBalance_engineNotRunning(t *testing.T) {
	t.Parallel()
	svc := NewEngineService(&engine.Engine{})
	_, err := svc.GetBalance(context.Background(), &enginev1.GetBalanceRequest{})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("code = %v", status.Code(err))
	}
}

func TestGetPosition_engineNotRunning(t *testing.T) {
	t.Parallel()
	svc := NewEngineService(&engine.Engine{})
	_, err := svc.GetPosition(context.Background(), &enginev1.GetPositionRequest{Symbol: "BTC/USDT"})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("code = %v", status.Code(err))
	}
}

func TestListPositions_engineNotRunning(t *testing.T) {
	t.Parallel()
	svc := NewEngineService(&engine.Engine{})
	_, err := svc.ListPositions(context.Background(), &enginev1.ListPositionsRequest{})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("code = %v", status.Code(err))
	}
}
