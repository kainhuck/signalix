package spot

import "testing"

func TestNewPublicEvent_ticker(t *testing.T) {
	snap := &TickerSnapshot{Pair: "BTC/USDT", Last: "1"}
	ev, err := NewPublicEvent(PublicTicker, snap)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := ev.Ticker()
	if !ok || got.Last != "1" {
		t.Fatalf("ticker: %+v", got)
	}
}

func TestNewPublicEvent_wrongPayload(t *testing.T) {
	_, err := NewPublicEvent(PublicTicker, &CandlestickSnapshot{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewUserEvent_balance(t *testing.T) {
	snap := &BalanceUpdateSnapshot{Currency: "USDT"}
	ev, err := NewUserEvent(UserBalanceUpdate, snap)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := ev.Balance()
	if !ok || got.Currency != "USDT" {
		t.Fatalf("balance: %+v", got)
	}
}

func TestNewUserEvent_order(t *testing.T) {
	snap := &OrderSnapshot{OrderID: "1"}
	ev, err := NewUserEvent(UserOrderUpdate, snap)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := ev.Order()
	if !ok || got.OrderID != "1" {
		t.Fatalf("order: %+v", got)
	}
}
