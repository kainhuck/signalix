package perp

import "testing"

func TestNewPublicEvent_Ticker(t *testing.T) {
	snap := &TickerSnapshot{Contract: "BTC/USDT", Last: "1"}
	ev, err := NewPublicEvent(PublicTicker, snap)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := ev.Ticker()
	if !ok || got.Contract != "BTC/USDT" {
		t.Fatalf("Ticker: %+v ok=%v", got, ok)
	}
}

func TestNewPublicEvent_NilPayload(t *testing.T) {
	_, err := NewPublicEvent(PublicTicker, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewPublicEvent_UnknownKind(t *testing.T) {
	_, err := NewPublicEvent(PublicKind("Trade"), &TickerSnapshot{Contract: "BTC/USDT"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewUserEvent_Order(t *testing.T) {
	ov := &OrderSnapshot{Contract: "ETH/USDT", ExchangeOrderID: "1"}
	ev, err := NewUserEvent(UserOrderUpdate, ov)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := ev.Order()
	if !ok || got.Contract != "ETH/USDT" {
		t.Fatalf("Order: %+v ok=%v", got, ok)
	}
}

func TestPublicEvent_Ticker_WrongKind(t *testing.T) {
	ev := &PublicEvent{Kind: PublicKind("Other"), Payload: &TickerSnapshot{}}
	if _, ok := ev.Ticker(); ok {
		t.Fatal("expected false")
	}
}

func TestNewPublicEvent_Candlestick(t *testing.T) {
	snap := &CandlestickSnapshot{
		Contract: "BTC/USDT",
		Interval: "1m",
		Close:    "100",
	}
	ev, err := NewPublicEvent(PublicCandlestick, snap)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := ev.Candlestick()
	if !ok || got.Interval != "1m" || got.Contract != "BTC/USDT" {
		t.Fatalf("Candlestick: %+v ok=%v", got, ok)
	}
}

func TestNewPublicEvent_Candlestick_Mismatch(t *testing.T) {
	_, err := NewPublicEvent(PublicCandlestick, &TickerSnapshot{Contract: "BTC/USDT"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewUserEvent_Trade(t *testing.T) {
	snap := &TradeSnapshot{
		Contract:        "BTC/USDT",
		TradeID:         "3335259",
		ExchangeOrderID: "4872460",
		Size:            "1",
		Price:           "40000.4",
	}
	ev, err := NewUserEvent(UserTradeUpdate, snap)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := ev.Trade()
	if !ok || got.TradeID != "3335259" || got.Contract != "BTC/USDT" {
		t.Fatalf("Trade: %+v ok=%v", got, ok)
	}
}

func TestNewUserEvent_Position(t *testing.T) {
	pv := &PositionSnapshot{Contract: "BTC/USDT", Side: PositionLong, Size: "3", EntryPrice: "40000"}
	ev, err := NewUserEvent(UserPositionUpdate, pv)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := ev.Position()
	if !ok || got.Contract != "BTC/USDT" || got.Size != "3" {
		t.Fatalf("Position: %+v ok=%v", got, ok)
	}
}

func TestNewUserEvent_Balance(t *testing.T) {
	snap := &BalanceUpdateSnapshot{Currency: "USDT", Balance: "100", ChangeType: "fee"}
	ev, err := NewUserEvent(UserBalanceUpdate, snap)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := ev.Balance()
	if !ok || got.Currency != "USDT" {
		t.Fatalf("Balance: %+v ok=%v", got, ok)
	}
}

func TestPublicEvent_Candlestick_WrongKind(t *testing.T) {
	ev := &PublicEvent{Kind: PublicTicker, Payload: &CandlestickSnapshot{}}
	if _, ok := ev.Candlestick(); ok {
		t.Fatal("expected false")
	}
}
