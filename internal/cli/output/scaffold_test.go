package output_test

import (
	"bytes"
	"strings"
	"testing"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/kainhuck/signalix/internal/cli/output"
)

func TestPrintTemplateList_table(t *testing.T) {
	var buf bytes.Buffer
	pr, err := output.NewPrinter("table", &buf)
	if err != nil {
		t.Fatal(err)
	}
	reply := &enginev1.ListTemplatesReply{
		Templates: []*enginev1.StrategyTemplate{
			{
				Id:              "trend",
				Description:     "Trend following example",
				DefaultSymbols:  []string{"BTC/USDT"},
				DefaultInterval: "5m",
			},
			{
				Id:              "blank",
				Description:     "Empty scaffold",
				DefaultInterval: "1m",
			},
		},
	}
	if err := pr.PrintTemplateList(reply); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "trend") || !strings.Contains(out, "blank") || !strings.Contains(out, "BTC/USDT") {
		t.Fatalf("unexpected: %s", out)
	}
}

func TestPrintTemplateList_json(t *testing.T) {
	var buf bytes.Buffer
	pr, err := output.NewPrinter("json", &buf)
	if err != nil {
		t.Fatal(err)
	}
	reply := &enginev1.ListTemplatesReply{
		Templates: []*enginev1.StrategyTemplate{{Id: "blank"}},
	}
	if err := pr.PrintTemplateList(reply); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "blank") {
		t.Fatalf("unexpected: %s", buf.String())
	}
}

func TestPrintCreateStrategy_table(t *testing.T) {
	var buf bytes.Buffer
	pr, err := output.NewPrinter("table", &buf)
	if err != nil {
		t.Fatal(err)
	}
	reply := &enginev1.CreateStrategyReply{
		Name:         "my_trend",
		Path:         "/home/user/.signalix/strategies/my_trend",
		CatalogCount: 3,
	}
	if err := pr.PrintCreateStrategy(reply); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "my_trend") || !strings.Contains(out, "catalog_count") || !strings.Contains(out, "3") {
		t.Fatalf("unexpected: %s", out)
	}
}

func TestPrintCreateStrategy_json(t *testing.T) {
	var buf bytes.Buffer
	pr, err := output.NewPrinter("json", &buf)
	if err != nil {
		t.Fatal(err)
	}
	reply := &enginev1.CreateStrategyReply{Name: "my_blank", CatalogCount: 1}
	if err := pr.PrintCreateStrategy(reply); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "my_blank") {
		t.Fatalf("unexpected: %s", buf.String())
	}
}
