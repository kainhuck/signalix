package grpc

import (
	"context"
	"testing"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/kainhuck/signalix/internal/app/engine"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetHealth_engineNotRunning(t *testing.T) {
	t.Parallel()
	svc := NewEngineService(&engine.Engine{})
	ctx := context.Background()

	reply, err := svc.GetHealth(ctx, &enginev1.GetHealthRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetStatus() != enginev1.HealthStatus_HEALTH_STATUS_UNHEALTHY {
		t.Fatalf("status = %v, want UNHEALTHY", reply.GetStatus())
	}
}

func TestPing_engineNotRunning(t *testing.T) {
	t.Parallel()
	svc := NewEngineService(&engine.Engine{})
	ctx := context.Background()

	reply, err := svc.Ping(ctx, &enginev1.PingRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetPong() != "pong" {
		t.Fatalf("pong = %q", reply.GetPong())
	}
}

func TestGetHealth_nilEngine(t *testing.T) {
	t.Parallel()
	svc := &EngineService{eng: nil}
	_, err := svc.GetHealth(context.Background(), &enginev1.GetHealthRequest{})
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %v, want Internal", status.Code(err))
	}
}
