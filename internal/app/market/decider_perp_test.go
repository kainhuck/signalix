package market

import (
	"context"
	"testing"

	"github.com/kainhuck/signalix/internal/app/decision"
	"github.com/kainhuck/signalix/internal/models"
)

func TestPerpDeciderNotConfigured(t *testing.T) {
	t.Parallel()
	var d MarketDecider = &perpDecider{}
	_, err := d.Decide(context.Background(), "s1", &models.Signal{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewPerpDeciderDelegatesToEngine(t *testing.T) {
	t.Parallel()
	de := decision.NewDecisionEngine(nil)
	d := NewPerpDecider(de)
	if d == nil {
		t.Fatal("expected decider")
	}
	_, err := d.Decide(context.Background(), "s1", &models.Signal{})
	if err == nil {
		t.Fatal("expected error from invalid signal")
	}
}
