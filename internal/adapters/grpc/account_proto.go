package grpc

import (
	"errors"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/kainhuck/signalix/internal/app/engine"
	"github.com/kainhuck/signalix/internal/app/projection"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func mapProjectionErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, projection.ErrProjectionNotReady) {
		return status.Error(codes.FailedPrecondition, "account projection not ready")
	}
	if errors.Is(err, engine.ErrAccountProjectionNotConfigured) {
		return status.Error(codes.FailedPrecondition, "account projection not configured")
	}
	return status.Errorf(codes.Internal, "account snapshot: %v", err)
}

func balanceToProto(b *perp.BalanceView) *enginev1.Balance {
	if b == nil {
		return nil
	}
	return &enginev1.Balance{
		Currency:        b.Currency,
		Total:           b.Total,
		Available:       b.Available,
		Frozen:          b.Frozen,
		UpdatedAtUnixMs: b.UpdatedAt.UnixMilli(),
	}
}

func positionToProto(p *perp.PositionSnapshot) *enginev1.Position {
	if p == nil {
		return nil
	}
	return &enginev1.Position{
		Symbol:          string(p.Contract),
		Side:            string(p.Side),
		Size:            p.Size,
		EntryPrice:      p.EntryPrice,
		MarkPrice:       p.MarkPrice,
		UnrealizedPnl:   p.UnrealizedPnl,
		Leverage:        int32(p.Leverage),
		UpdatedAtUnixMs: p.UpdatedAt.UnixMilli(),
	}
}
