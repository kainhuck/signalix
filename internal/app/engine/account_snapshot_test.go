package engine

import (
	"errors"
	"testing"
	"time"

	"github.com/kainhuck/signalix/internal/app/projection"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/testutil"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	spotex "github.com/kainhuck/signalix/pkg/exchange/spot"
)

func TestBalanceSnapshot_projectionNotConfigured(t *testing.T) {
	t.Parallel()
	e := &Engine{}
	_, err := e.BalanceSnapshot()
	if !errors.Is(err, ErrAccountProjectionNotConfigured) {
		t.Fatalf("err = %v", err)
	}
}

func TestBalanceSnapshot_notReady(t *testing.T) {
	t.Parallel()
	e := newTestEngine(t, t.TempDir(), testutil.NewStubExchange())
	_, err := e.BalanceSnapshot()
	if !errors.Is(err, projection.ErrProjectionNotReady) {
		t.Fatalf("err = %v", err)
	}
}

func TestBalanceSnapshot_ok(t *testing.T) {
	t.Parallel()
	e := newTestEngine(t, t.TempDir(), testutil.NewStubExchange())
	if err := e.accountProjection.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	bal, err := e.BalanceSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if bal == nil || bal.Currency != "USDT" || bal.Available != "10000" {
		t.Fatalf("balance = %+v", bal)
	}
}

func TestPositionSnapshot_noPosition(t *testing.T) {
	t.Parallel()
	e := newTestEngine(t, t.TempDir(), testutil.NewStubExchange())
	if err := e.accountProjection.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	pos, err := e.PositionSnapshot("BTC/USDT")
	if err != nil {
		t.Fatal(err)
	}
	if pos != nil {
		t.Fatalf("expected nil position, got %+v", pos)
	}
}

func TestAllPositionsSnapshot_withPositions(t *testing.T) {
	t.Parallel()
	ex := testutil.NewStubExchange()
	ex.PositionsList = []*perp.PositionSnapshot{
		{
			Contract: perp.Contract("BTC/USDT"),
			Side:     perp.PositionLong,
			Size:     "1",
		},
	}
	e := newTestEngine(t, t.TempDir(), ex)
	if err := e.accountProjection.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	list, err := e.AllPositionsSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Symbol != "BTC/USDT" {
		t.Fatalf("list = %+v", list)
	}
}

func TestAllPositionsSnapshot_empty(t *testing.T) {
	t.Parallel()
	e := newTestEngine(t, t.TempDir(), testutil.NewStubExchange())
	if err := e.accountProjection.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	list, err := e.AllPositionsSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %d", len(list))
	}
}

func TestBalanceSnapshotForMarket_spot(t *testing.T) {
	t.Parallel()
	ex := testutil.NewStubSpotExchange()
	ex.BalancesList = append(ex.BalancesList, &spotex.BalanceView{
		Currency:  "BTC",
		Total:     "0.25",
		Available: "0.2",
		Frozen:    "0.05",
		UpdatedAt: time.Unix(10, 0),
	})
	e := &Engine{markets: testSpotMarkets(t, ex)}

	bal, err := e.BalanceSnapshotForMarket(models.MarketSpot, "btc")
	if err != nil {
		t.Fatal(err)
	}
	if bal.Currency != "BTC" || bal.Total != "0.25" || bal.Available != "0.2" {
		t.Fatalf("balance = %+v", bal)
	}
}

func TestAllPositionsSnapshotForMarket_spot(t *testing.T) {
	t.Parallel()
	ex := testutil.NewStubSpotExchange()
	ex.BalancesList = append(ex.BalancesList, &spotex.BalanceView{
		Currency:  "BTC",
		Total:     "0.25",
		Available: "0.2",
		UpdatedAt: time.Unix(10, 0),
	})
	e := &Engine{markets: testSpotMarkets(t, ex)}

	list, err := e.AllPositionsSnapshotForMarket(models.MarketSpot)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Market != models.MarketSpot || list[0].Symbol != "BTC/USDT" || list[0].Size != "0.25" {
		t.Fatalf("list = %+v", list)
	}
}
