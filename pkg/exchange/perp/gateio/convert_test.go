package gateio

import (
	"testing"

	"github.com/gate/gateapi-go/v7"
)

func TestOrderViewFromFutures_ClientID(t *testing.T) {
	t.Parallel()

	ov := orderViewFromFutures(gateapi.FuturesOrder{
		Id:       1,
		Contract: "BTC_USDT",
		Text:     "t-abc123",
	})
	if ov.ClientID != "t-abc123" {
		t.Fatalf("ClientID = %q, want t-abc123", ov.ClientID)
	}
}
