package output_test

import (
	"bytes"
	"strings"
	"testing"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/kainhuck/signalix/internal/cli/output"
)

func TestPrintBalance_table(t *testing.T) {
	var buf bytes.Buffer
	pr, err := output.NewPrinter("table", &buf)
	if err != nil {
		t.Fatal(err)
	}
	reply := &enginev1.GetBalanceReply{
		Balance: &enginev1.Balance{
			Market:          "spot",
			Currency:        "USDT",
			Total:           "1000",
			Available:       "900",
			Frozen:          "100",
			UpdatedAtUnixMs: 1_700_000_000_000,
		},
	}
	if err := pr.PrintBalance(reply); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "spot") || !strings.Contains(out, "USDT") || !strings.Contains(out, "1000") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestPrintPosition_empty(t *testing.T) {
	var buf bytes.Buffer
	pr, err := output.NewPrinter("table", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if err := pr.PrintPosition(&enginev1.GetPositionReply{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "(none)") {
		t.Fatalf("expected (none): %s", buf.String())
	}
}

func TestPrintPositionList_json(t *testing.T) {
	var buf bytes.Buffer
	pr, err := output.NewPrinter("json", &buf)
	if err != nil {
		t.Fatal(err)
	}
	reply := &enginev1.ListPositionsReply{
		Positions: []*enginev1.Position{
			{Market: "spot", Symbol: "BTC/USDT", Side: "long", Size: "1"},
		},
	}
	if err := pr.PrintPositionList(reply); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "spot") || !strings.Contains(buf.String(), "BTC/USDT") {
		t.Fatalf("unexpected json: %s", buf.String())
	}
}

func TestPrintOrderList_table(t *testing.T) {
	var buf bytes.Buffer
	pr, err := output.NewPrinter("table", &buf)
	if err != nil {
		t.Fatal(err)
	}
	reply := &enginev1.ListOpenOrdersReply{
		Orders: []*enginev1.Order{
			{Id: "a", Market: "perp", UpdatedAtUnixMs: 100, Symbol: "BTC/USDT", Status: "open"},
			{Id: "b", Market: "spot", UpdatedAtUnixMs: 200, Symbol: "ETH/USDT", Status: "open"},
		},
	}
	if err := pr.PrintOrderList(reply); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Index(out, "b") > strings.Index(out, "a") {
		t.Fatalf("expected updated_at desc (b before a): %s", out)
	}
	if !strings.Contains(out, "spot") || !strings.Contains(out, "perp") {
		t.Fatalf("expected market column: %s", out)
	}
}

func TestPrintOrderEvent_ndjson(t *testing.T) {
	var buf bytes.Buffer
	pr, err := output.NewPrinter("json", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if err := pr.PrintOrderEvent(&enginev1.Order{Id: "ord-1", Market: "spot", Symbol: "BTC/USDT"}); err != nil {
		t.Fatal(err)
	}
	line := buf.String()
	if !strings.Contains(line, `"id"`) || !strings.Contains(line, "ord-1") || !strings.Contains(line, "spot") || !strings.HasSuffix(line, "\n") {
		t.Fatalf("unexpected ndjson: %q", line)
	}
}

func TestPrintOrderCancel_json(t *testing.T) {
	var buf bytes.Buffer
	pr, err := output.NewPrinter("json", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if err := pr.PrintOrderCancel("ord-1"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "cancelled") {
		t.Fatalf("unexpected: %s", buf.String())
	}
}
