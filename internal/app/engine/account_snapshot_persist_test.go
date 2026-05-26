package engine

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

func TestEncodeAccountSnapshotRow(t *testing.T) {
	at := time.Date(2026, 5, 26, 10, 0, 0, 0, time.UTC)
	bal := &perp.BalanceView{
		Currency:  "USDT",
		Total:     "12345.67",
		Available: "10000",
		Frozen:    "2345.67",
		UpdatedAt: time.Unix(1710000000, 0),
	}
	positions := []*perp.PositionSnapshot{
		{Contract: "BTC/USDT", Side: perp.PositionShort, Size: "2", UpdatedAt: time.Unix(2, 0)},
		{Contract: "ETH/USDT", Side: perp.PositionLong, Size: "1", UpdatedAt: time.Unix(1, 0)},
	}

	row, err := encodeAccountSnapshotRow(bal, positions, 5, at)
	if err != nil {
		t.Fatal(err)
	}
	if row.Currency != "USDT" || row.TotalEquity != "12345.67" {
		t.Fatalf("row: %+v", row)
	}
	if row.Revision != 5 || !row.SnapshotAt.Equal(at) {
		t.Fatalf("snapshot meta: %+v", row)
	}

	var balMap map[string]interface{}
	if err := json.Unmarshal(row.BalanceJSON, &balMap); err != nil {
		t.Fatal(err)
	}
	if balMap["total"] != "12345.67" {
		t.Fatalf("balance json: %+v", balMap)
	}

	var posList []map[string]interface{}
	if err := json.Unmarshal(row.PositionsJSON, &posList); err != nil {
		t.Fatal(err)
	}
	if len(posList) != 2 {
		t.Fatalf("positions len: %d", len(posList))
	}
	if posList[0]["contract"] != "BTC/USDT" || posList[1]["contract"] != "ETH/USDT" {
		t.Fatalf("positions order: %+v", posList)
	}
}

func TestEncodeAccountSnapshotRow_emptyPositions(t *testing.T) {
	row, err := encodeAccountSnapshotRow(&perp.BalanceView{
		Currency: "USDT",
		Total:    "1",
	}, nil, 1, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if string(row.PositionsJSON) != "[]" {
		t.Fatalf("positions json: %s", row.PositionsJSON)
	}
}
