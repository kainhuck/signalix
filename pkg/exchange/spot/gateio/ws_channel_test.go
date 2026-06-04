package gateio

import (
	"testing"

	"github.com/kainhuck/signalix/pkg/exchange/spot"
)

func TestExpandPairOnly(t *testing.T) {
	h := newWSHub(&Client{})
	payloads, err := expandPairOnly(h, &spot.Subscription{
		Channel: WSChannelTickers,
		Pairs:   []spot.Pair{"BTC/USDT", "ETH/USDT"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(payloads) != 2 {
		t.Fatalf("len=%d", len(payloads))
	}
	if payloads[0][0] != "BTC_USDT" || payloads[1][0] != "ETH_USDT" {
		t.Fatalf("payloads=%v", payloads)
	}
}

func TestExpandPairOnly_requiresPairs(t *testing.T) {
	h := newWSHub(&Client{})
	_, err := expandPairOnly(h, &spot.Subscription{Channel: WSChannelOrders})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExpandCandlesticks(t *testing.T) {
	h := newWSHub(&Client{})
	payloads, err := expandCandlesticks(h, &spot.Subscription{
		Channel: WSChannelCandlesticks,
		Pairs:   []spot.Pair{"BTC/USDT"},
		Payload: []string{"1m"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(payloads) != 1 || payloads[0][0] != "1m" || payloads[0][1] != "BTC_USDT" {
		t.Fatalf("payloads=%v", payloads)
	}
}

func TestExpandCandlesticks_requiresInterval(t *testing.T) {
	h := newWSHub(&Client{})
	_, err := expandCandlesticks(h, &spot.Subscription{
		Channel: WSChannelCandlesticks,
		Pairs:   []spot.Pair{"BTC/USDT"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExpandEmptyPayload(t *testing.T) {
	h := newWSHub(&Client{})
	payloads, err := expandEmptyPayload(h, &spot.Subscription{Channel: WSChannelBalances})
	if err != nil {
		t.Fatal(err)
	}
	if len(payloads) != 1 || len(payloads[0]) != 0 {
		t.Fatalf("payloads=%v", payloads)
	}
}
