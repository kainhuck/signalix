package strategy

import "testing"

func TestResolveInterval(t *testing.T) {
	t.Parallel()

	cfg := StrategyConfig{Interval: "5m"}
	got, err := ResolveInterval(cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "5m" {
		t.Fatalf("got %q want 5m", got)
	}

	cfg = StrategyConfig{}
	got, err = ResolveInterval(cfg, "1m")
	if err != nil {
		t.Fatal(err)
	}
	if got != "1m" {
		t.Fatalf("got %q want 1m", got)
	}

	_, err = ResolveInterval(StrategyConfig{}, "")
	if err == nil {
		t.Fatal("expected error for missing interval")
	}

	_, err = ResolveInterval(StrategyConfig{Interval: "2h"}, "")
	if err == nil {
		t.Fatal("expected error for invalid interval")
	}
}
