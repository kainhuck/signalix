package perp

import (
	"context"
	"testing"

	"github.com/kainhuck/signalix/internal/models"
)

func TestPerpRiskBuildRiskContext_nilOrder(t *testing.T) {
	t.Parallel()
	r := NewPerpRisk(PerpRiskConfig{})
	_, err := r.BuildRiskContext(context.Background(), "s1", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPerpRiskBuildRiskContext_flatSkipsNotional(t *testing.T) {
	t.Parallel()
	r := NewPerpRisk(PerpRiskConfig{})
	rc, err := r.BuildRiskContext(context.Background(), "s1", &models.Signal{
		Symbol:    "BTC/USDT",
		Direction: models.DirectionFlat,
	}, &models.Order{Symbol: "BTC/USDT", Size: "1"})
	if err != nil {
		t.Fatal(err)
	}
	if !rc.NotionalAvailable {
		t.Fatal("expected NotionalAvailable for flat")
	}
	if rc.QuoteCcy != perpQuoteCcy {
		t.Fatalf("QuoteCcy = %q, want %q", rc.QuoteCcy, perpQuoteCcy)
	}
	if rc.OpensExposure {
		t.Fatal("flat should not open exposure")
	}
}
