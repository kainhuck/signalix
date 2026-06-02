package grpc

import (
	"errors"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/kainhuck/signalix/internal/app/engine"
	"github.com/kainhuck/signalix/internal/app/projection"
	"github.com/kainhuck/signalix/internal/models"
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

func balanceToProto(b *models.BalanceView) *enginev1.Balance {
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

func positionToProto(p *models.PositionView) *enginev1.Position {
	if p == nil {
		return nil
	}
	lev := int32(0)
	if p.Leverage != "" {
		if n, err := parseLeverageInt32(p.Leverage); err == nil {
			lev = n
		}
	}
	return &enginev1.Position{
		Symbol:          p.Symbol,
		Side:            p.Side,
		Size:            p.Size,
		EntryPrice:      p.EntryPrice,
		MarkPrice:       p.MarkPrice,
		UnrealizedPnl:   p.UnrealizedPnl,
		Leverage:        lev,
		UpdatedAtUnixMs: p.UpdatedAt.UnixMilli(),
	}
}

func parseLeverageInt32(s string) (int32, error) {
	var n int64
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, errors.New("invalid leverage")
		}
		n = n*10 + int64(c-'0')
	}
	return int32(n), nil
}
