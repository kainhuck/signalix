package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
)

func TestPrintStrategyLogList_table(t *testing.T) {
	var buf bytes.Buffer
	pr, err := NewPrinter("table", &buf)
	if err != nil {
		t.Fatal(err)
	}
	reply := &enginev1.ListStrategyLogsReply{
		Logs: []*enginev1.StrategyLog{
			{
				Id:              1,
				StrategyName:    "trend",
				Level:           "info",
				Message:         "hello",
				CreatedAtUnixMs: 1710000000000,
			},
		},
	}
	if err := pr.PrintStrategyLogList(reply); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	outLower := strings.ToLower(out)
	for _, want := range []string{"strategy", "level", "trend", "hello"} {
		if !strings.Contains(outLower, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestPrintStrategyLogList_json(t *testing.T) {
	var buf bytes.Buffer
	pr, err := NewPrinter("json", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if err := pr.PrintStrategyLogList(&enginev1.ListStrategyLogsReply{
		Logs: []*enginev1.StrategyLog{{Id: 2, StrategyName: "s", Level: "warn", Message: "m"}},
	}); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, buf.String())
	}
}

func TestPrintStrategyLogEvent_tableAndNDJSON(t *testing.T) {
	log := &enginev1.StrategyLog{
		Id:              3,
		StrategyName:    "alpha",
		Level:           "error",
		Message:         "boom",
		CreatedAtUnixMs: 1710000000000,
	}

	var tableBuf bytes.Buffer
	tablePr, err := NewPrinter("table", &tableBuf)
	if err != nil {
		t.Fatal(err)
	}
	if err := tablePr.PrintStrategyLogEventHeader(); err != nil {
		t.Fatal(err)
	}
	if err := tablePr.PrintStrategyLogEvent(log); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tableBuf.String(), "alpha\terror") && !strings.Contains(tableBuf.String(), "alpha") {
		t.Fatalf("table output: %q", tableBuf.String())
	}

	var jsonBuf bytes.Buffer
	jsonPr, err := NewPrinter("json", &jsonBuf)
	if err != nil {
		t.Fatal(err)
	}
	if err := jsonPr.PrintStrategyLogEvent(log); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(jsonBuf.String(), `"strategy_name"`) {
		t.Fatalf("ndjson: %q", jsonBuf.String())
	}
}

func TestPrintAccountSnapshotList_table(t *testing.T) {
	var buf bytes.Buffer
	pr, err := NewPrinter("table", &buf)
	if err != nil {
		t.Fatal(err)
	}
	reply := &enginev1.ListAccountSnapshotsReply{
		Snapshots: []*enginev1.AccountSnapshot{
			{
				Summary: &enginev1.AccountSnapshotSummary{
					Id:               10,
					SnapshotAtUnixMs: 1710000000000,
					Revision:         2,
					Currency:         "USDT",
					TotalEquity:      "100.5",
				},
			},
		},
	}
	if err := pr.PrintAccountSnapshotList(reply); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	outLower := strings.ToLower(out)
	for _, want := range []string{"total equity", "100.5", "usdt"} {
		if !strings.Contains(outLower, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestPrintLatestAccountSnapshot_empty(t *testing.T) {
	var buf bytes.Buffer
	pr, err := NewPrinter("table", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if err := pr.PrintLatestAccountSnapshot(&enginev1.GetLatestAccountSnapshotReply{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "(none)") {
		t.Fatalf("got %q", buf.String())
	}
}

func TestPrintLatestAccountSnapshot_withData(t *testing.T) {
	var buf bytes.Buffer
	pr, err := NewPrinter("json", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if err := pr.PrintLatestAccountSnapshot(&enginev1.GetLatestAccountSnapshotReply{
		Snapshot: &enginev1.AccountSnapshot{
			Summary: &enginev1.AccountSnapshotSummary{
				Id:               1,
				SnapshotAtUnixMs: 1710000000000,
				TotalEquity:      "99",
				Currency:         "USDT",
				Revision:         1,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"total_equity"`) {
		t.Fatalf("got %q", buf.String())
	}
}
