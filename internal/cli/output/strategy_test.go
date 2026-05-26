package output_test

import (
	"bytes"
	"strings"
	"testing"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/kainhuck/signalix/internal/cli/output"
)

func sampleStrategies() *enginev1.ListStrategiesReply {
	return &enginev1.ListStrategiesReply{
		Strategies: []*enginev1.StrategySummary{
			{
				Name:               "alpha",
				Enabled:            true,
				Symbols:            []string{"BTC/USDT"},
				Running:            true,
				CrashCount:         1,
				CrashCountInWindow: 1,
				CircuitOpen:        false,
				RestartBackoffSec:  0,
			},
			{
				Name:               "beta",
				Enabled:            false,
				Symbols:            []string{"ETH/USDT"},
				Running:            false,
				CrashCount:         0,
				CrashCountInWindow: 0,
				CircuitOpen:        true,
				RestartBackoffSec:  2.5,
			},
		},
	}
}

func TestPrintStrategyList_table(t *testing.T) {
	var buf bytes.Buffer
	pr, err := output.NewPrinter("table", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if err := pr.PrintStrategyList(sampleStrategies()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"alpha", "beta", "BTC/USDT", "CIRCUIT OPEN"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	// sorted: alpha before beta
	if strings.Index(out, "alpha") > strings.Index(out, "beta") {
		t.Fatalf("expected alpha before beta:\n%s", out)
	}
}

func TestPrintStrategyList_json(t *testing.T) {
	var buf bytes.Buffer
	pr, err := output.NewPrinter("json", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if err := pr.PrintStrategyList(sampleStrategies()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"strategies"`) || !strings.Contains(out, `"alpha"`) {
		t.Fatalf("unexpected json: %s", out)
	}
}

func TestPrintStrategyStatus_table(t *testing.T) {
	var buf bytes.Buffer
	pr, err := output.NewPrinter("table", &buf)
	if err != nil {
		t.Fatal(err)
	}
	reply := &enginev1.GetStrategyStatusReply{
		Status: &enginev1.StrategySummary{
			Name:                "alpha",
			Enabled:             true,
			Symbols:             []string{"BTC/USDT"},
			Running:             true,
			LastHeartbeatUnixMs: 1_700_000_000_000,
			CrashCount:          2,
			CrashCountInWindow:  1,
			AutoRestartEnabled:  true,
			RestartBackoffSec:   0,
			CircuitOpen:         false,
		},
	}
	if err := pr.PrintStrategyStatus(reply); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "last_heartbeat") || !strings.Contains(out, "auto_restart_enabled") {
		t.Fatalf("unexpected table: %s", out)
	}
}

func TestPrintStrategyAction_json(t *testing.T) {
	var buf bytes.Buffer
	pr, err := output.NewPrinter("json", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if err := pr.PrintStrategyAction("alpha", "started"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"action"`) || !strings.Contains(out, `"started"`) {
		t.Fatalf("unexpected json: %s", out)
	}
}

func TestPrintReloadResult_table(t *testing.T) {
	var buf bytes.Buffer
	pr, err := output.NewPrinter("table", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if err := pr.PrintReloadResult(3); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "catalog_count") || !strings.Contains(buf.String(), "3") {
		t.Fatalf("unexpected table: %s", buf.String())
	}
}
