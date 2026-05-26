package output_test

import (
	"bytes"
	"strings"
	"testing"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/kainhuck/signalix/internal/cli/output"
)

func TestPrintTicker_table(t *testing.T) {
	var buf bytes.Buffer
	pr, err := output.NewPrinter("table", &buf)
	if err != nil {
		t.Fatal(err)
	}
	reply := &enginev1.GetTickerReply{
		Ticker: &enginev1.Ticker{
			Symbol:          "BTC/USDT",
			Last:            "50000",
			MarkPrice:       "50001",
			ChangePct_24H:   "1.2",
			TimestampUnixMs: 1_700_000_000_000,
		},
	}
	if err := pr.PrintTicker(reply); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "BTC/USDT") || !strings.Contains(out, "50000") {
		t.Fatalf("unexpected: %s", out)
	}
}

func TestPrintKlines_table_empty(t *testing.T) {
	var buf bytes.Buffer
	pr, err := output.NewPrinter("table", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if err := pr.PrintKlines(&enginev1.GetKlinesReply{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "TIMESTAMP") {
		t.Fatalf("expected header: %s", buf.String())
	}
}

func TestPrintKlines_json(t *testing.T) {
	var buf bytes.Buffer
	pr, err := output.NewPrinter("json", &buf)
	if err != nil {
		t.Fatal(err)
	}
	reply := &enginev1.GetKlinesReply{
		Klines: []*enginev1.Kline{
			{Symbol: "BTC/USDT", Interval: "5m", Close: "100", TimestampUnixSec: 1_700_000_000},
		},
	}
	if err := pr.PrintKlines(reply); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"close"`) {
		t.Fatalf("unexpected json: %s", buf.String())
	}
}

func TestPrintTickerList_table(t *testing.T) {
	var buf bytes.Buffer
	pr, err := output.NewPrinter("table", &buf)
	if err != nil {
		t.Fatal(err)
	}
	reply := &enginev1.ListTickersReply{
		Tickers: []*enginev1.Ticker{
			{Symbol: "ETH/USDT", Last: "3000"},
			{Symbol: "BTC/USDT", Last: "50000"},
		},
	}
	if err := pr.PrintTickerList(reply); err != nil {
		t.Fatal(err)
	}
	if strings.Index(buf.String(), "BTC/USDT") > strings.Index(buf.String(), "ETH/USDT") {
		t.Fatalf("expected BTC before ETH: %s", buf.String())
	}
}

func TestPrintActivateKillSwitch_table(t *testing.T) {
	var buf bytes.Buffer
	pr, err := output.NewPrinter("table", &buf)
	if err != nil {
		t.Fatal(err)
	}
	reply := &enginev1.ActivateKillSwitchReply{
		Status: &enginev1.KillSwitchStatus{
			Active:            true,
			Reason:            "manual",
			ActivatedAtUnixMs: 1_700_000_000_000,
		},
		CancelAttempted: 2,
		CancelFailed:    1,
	}
	if err := pr.PrintActivateKillSwitch(reply); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"cancel_attempted", "2", "manual"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %s", want, out)
		}
	}
}

func TestPrintKillSwitchStatus_json(t *testing.T) {
	var buf bytes.Buffer
	pr, err := output.NewPrinter("json", &buf)
	if err != nil {
		t.Fatal(err)
	}
	reply := &enginev1.GetKillSwitchStatusReply{
		Status: &enginev1.KillSwitchStatus{Active: false},
	}
	if err := pr.PrintKillSwitchStatus(reply); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"status"`) {
		t.Fatalf("unexpected: %s", buf.String())
	}
}
