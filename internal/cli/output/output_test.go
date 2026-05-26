package output_test

import (
	"bytes"
	"strings"
	"testing"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/kainhuck/signalix/internal/cli/output"
)

func TestPrinter_pingJSON(t *testing.T) {
	var buf bytes.Buffer
	pr, err := output.NewPrinter("json", &buf)
	if err != nil {
		t.Fatal(err)
	}
	reply := &enginev1.PingReply{Pong: "ok", ServerTimeUnixMs: 1_700_000_000_000}
	if err := pr.PrintPing(reply); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"pong"`) || !strings.Contains(out, `"ok"`) {
		t.Fatalf("unexpected json: %s", out)
	}
}

func TestPrinter_pingYAML(t *testing.T) {
	var buf bytes.Buffer
	pr, err := output.NewPrinter("yaml", &buf)
	if err != nil {
		t.Fatal(err)
	}
	reply := &enginev1.PingReply{Pong: "ok", ServerTimeUnixMs: 1_700_000_000_000}
	if err := pr.PrintPing(reply); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "pong:") || !strings.Contains(out, "ok") {
		t.Fatalf("unexpected yaml: %s", out)
	}
}

func TestPrinter_pingTable(t *testing.T) {
	var buf bytes.Buffer
	pr, err := output.NewPrinter("table", &buf)
	if err != nil {
		t.Fatal(err)
	}
	reply := &enginev1.PingReply{Pong: "ok", ServerTimeUnixMs: 1_700_000_000_000}
	if err := pr.PrintPing(reply); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "pong") || !strings.Contains(out, "ok") {
		t.Fatalf("unexpected table: %s", out)
	}
}

func TestHealthProbeOK(t *testing.T) {
	if !output.HealthProbeOK(enginev1.HealthStatus_HEALTH_STATUS_HEALTHY) {
		t.Fatal("healthy should pass probe")
	}
	if output.HealthProbeOK(enginev1.HealthStatus_HEALTH_STATUS_DEGRADED) {
		t.Fatal("degraded should fail probe")
	}
}

func TestNewPrinter_invalid(t *testing.T) {
	if _, err := output.NewPrinter("xml", &bytes.Buffer{}); err == nil {
		t.Fatal("expected error")
	}
}
