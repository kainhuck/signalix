package gateio

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kainhuck/signalix/pkg/exchange/spot"
)

func TestClient_connectREST_and_ListPairMeta(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/spot/accounts":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"currency":"USDT","available":"100","locked":"0"}]`))
		case r.URL.Path == "/spot/currency_pairs":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":"BTC_USDT","trade_status":"tradable","amount_precision":4,"precision":2}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewClient("key", "secret", WithRESTBasePath(srv.URL))
	if err := c.Connect(context.Background(), spot.DefaultConnectREST()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	metas, err := c.ListPairMeta(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 1 || metas[0].Pair != "BTC/USDT" {
		t.Fatalf("metas: %+v", metas)
	}
}

func spotTestJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(body))
}

func TestClient_Place_limit(t *testing.T) {
	var capturedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/spot/accounts":
			spotTestJSON(w, `[]`)
		case "/spot/currency_pairs":
			spotTestJSON(w, `[{"id":"BTC_USDT","trade_status":"tradable"}]`)
		case "/spot/orders":
			b, _ := io.ReadAll(r.Body)
			capturedBody = string(b)
			spotTestJSON(w, `{"id":"99","currency_pair":"BTC_USDT","side":"buy","type":"limit","amount":"0.01","price":"50000","status":"open","left":"0.01"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewClient("key", "secret", WithRESTBasePath(srv.URL))
	if err := c.Connect(context.Background(), spot.DefaultConnectREST()); err != nil {
		t.Fatal(err)
	}
	price := "50000"
	ov, err := c.Place(context.Background(), &spot.PlaceRequest{
		Pair:  spot.CanonicalPair("BTC/USDT"),
		Side:  spot.SideBuy,
		Type:  spot.OrderTypeLimit,
		Size:  "0.01",
		Price: &price,
	})
	if err != nil {
		t.Fatal(err)
	}
	if ov.OrderID != "99" || ov.Pair != "BTC/USDT" {
		t.Fatalf("order: %+v", ov)
	}
	if !strings.Contains(capturedBody, `"amount":"0.01"`) || !strings.Contains(capturedBody, `"account":"spot"`) {
		t.Fatalf("body: %s", capturedBody)
	}
}

func TestClient_Balances_and_Balance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/spot/accounts":
			spotTestJSON(w, `[{"currency":"BTC","available":"1","locked":"0.1"},{"currency":"USDT","available":"0","locked":"0"}]`)
		case "/spot/currency_pairs":
			spotTestJSON(w, `[{"id":"BTC_USDT"}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewClient("k", "s", WithRESTBasePath(srv.URL))
	if err := c.Connect(context.Background(), spot.DefaultConnectREST()); err != nil {
		t.Fatal(err)
	}
	bals, err := c.Balances(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(bals) != 1 || bals[0].Currency != "BTC" {
		t.Fatalf("balances: %+v", bals)
	}
	zero, err := c.Balance(context.Background(), "USDT")
	if err != nil {
		t.Fatal(err)
	}
	if zero.Total != "0" {
		t.Fatalf("zero balance: %+v", zero)
	}
}

func TestClient_GetOrder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/spot/accounts":
			spotTestJSON(w, `[]`)
		case r.URL.Path == "/spot/currency_pairs":
			spotTestJSON(w, `[{"id":"BTC_USDT"}]`)
		case strings.HasPrefix(r.URL.Path, "/spot/orders/"):
			spotTestJSON(w, `{"id":"42","currency_pair":"BTC_USDT","side":"sell","type":"limit","amount":"0.1","price":"60000","status":"open","left":"0.1"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewClient("k", "s", WithRESTBasePath(srv.URL))
	if err := c.Connect(context.Background(), spot.DefaultConnectREST()); err != nil {
		t.Fatal(err)
	}
	ov, err := c.GetOrder(context.Background(), spot.CanonicalPair("BTC/USDT"), "42")
	if err != nil {
		t.Fatal(err)
	}
	if ov.OrderID != "42" || ov.Side != spot.SideSell {
		t.Fatalf("order: %+v", ov)
	}
}
