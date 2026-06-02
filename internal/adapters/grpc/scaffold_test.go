package grpc

import (
	"context"
	"testing"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/kainhuck/signalix/internal/app/engine"
	"github.com/kainhuck/signalix/internal/testutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCreateStrategy_engineNotRunning(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	markets, err := engine.NewTestPerpMarkets(testutil.NewStubExchange())
	if err != nil {
		t.Fatal(err)
	}
	e := engine.NewEngine(dir, markets, engine.BuildParams{})
	svc := NewEngineService(e)
	_, err = svc.CreateStrategy(context.Background(), &enginev1.CreateStrategyRequest{
		Name:       "x",
		TemplateId: "trend",
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("code = %v", status.Code(err))
	}
}

func TestListTemplates_ok(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	markets, err := engine.NewTestPerpMarkets(testutil.NewStubExchange())
	if err != nil {
		t.Fatal(err)
	}
	e := engine.NewEngine(dir, markets, engine.BuildParams{})
	svc := NewEngineService(e)
	reply, err := svc.ListTemplates(context.Background(), &enginev1.ListTemplatesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, tpl := range reply.GetTemplates() {
		ids[tpl.GetId()] = true
	}
	if !ids["trend"] || !ids["blank"] {
		t.Fatalf("templates = %+v", reply.GetTemplates())
	}
}
