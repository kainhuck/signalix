package gateio

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestWriteChannelEvent_public(t *testing.T) {
	var got map[string]interface{}
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		up := websocket.Upgrader{}
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		if err := conn.ReadJSON(&got); err != nil {
			t.Error(err)
			return
		}
		close(done)
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	c := NewClient("key", "secret")
	h := newWSHub(c)
	h.wsURL = wsURL
	if err := h.connect(t.Context()); err != nil {
		t.Fatal(err)
	}
	spec, _ := wsChannelSpecFor(WSChannelTickers)
	conn, err := h.requireConn()
	if err != nil {
		t.Fatal(err)
	}
	if err := h.writeChannelEvent(conn, spec, "subscribe", []string{"BTC_USDT"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
	if got["channel"] != WSChannelTickers || got["event"] != "subscribe" {
		t.Fatalf("got=%v", got)
	}
	if _, ok := got["auth"]; ok {
		t.Fatal("public frame must not have auth")
	}
	payload, ok := got["payload"].([]interface{})
	if !ok || len(payload) != 1 || payload[0] != "BTC_USDT" {
		t.Fatalf("payload=%v", got["payload"])
	}
}

func TestWriteChannelEvent_private(t *testing.T) {
	var got map[string]interface{}
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		up := websocket.Upgrader{}
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		if err := conn.ReadJSON(&got); err != nil {
			t.Error(err)
			return
		}
		close(done)
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	c := NewClient("test-key", "test-secret")
	h := newWSHub(c)
	h.wsURL = wsURL
	if err := h.connect(t.Context()); err != nil {
		t.Fatal(err)
	}
	spec, _ := wsChannelSpecFor(WSChannelBalances)
	conn, err := h.requireConn()
	if err != nil {
		t.Fatal(err)
	}
	if err := h.writeChannelEvent(conn, spec, "subscribe", nil); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
	auth, ok := got["auth"].(map[string]interface{})
	if !ok {
		b, _ := json.Marshal(got)
		t.Fatalf("missing auth: %s", b)
	}
	if auth["method"] != "api_key" || auth["KEY"] != "test-key" {
		t.Fatalf("auth=%v", auth)
	}
	if sign, _ := auth["SIGN"].(string); sign == "" {
		t.Fatal("empty SIGN")
	}
}

func TestWsSign(t *testing.T) {
	sign := wsSign("secret", WSChannelOrders, "subscribe", 1611541000)
	if sign == "" || len(sign) != 128 {
		t.Fatalf("sign=%q len=%d", sign, len(sign))
	}
}
