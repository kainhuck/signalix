package grpc

import (
	"context"
	"testing"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/kainhuck/signalix/internal/app/engine"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestKillSwitchRPCRequiresRunningEngine(t *testing.T) {
	t.Parallel()
	svc := NewEngineService(&engine.Engine{})
	ctx := context.Background()

	_, err := svc.GetKillSwitchStatus(ctx, &enginev1.GetKillSwitchStatusRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("code = %v, want FailedPrecondition", status.Code(err))
	}

	_, err = svc.ActivateKillSwitch(ctx, &enginev1.ActivateKillSwitchRequest{Reason: "x"})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("Activate code = %v", status.Code(err))
	}

	_, err = svc.DeactivateKillSwitch(ctx, &enginev1.DeactivateKillSwitchRequest{})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("Deactivate code = %v", status.Code(err))
	}
}
