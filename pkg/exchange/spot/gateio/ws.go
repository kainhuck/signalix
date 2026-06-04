package gateio

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kainhuck/signalix/pkg/exchange/spot"

	"github.com/gorilla/websocket"
)

type wsHub struct {
	c *Client

	wsURL string

	mu   sync.RWMutex
	conn *websocket.Conn

	hubCtx    context.Context
	hubCancel context.CancelFunc

	wg        sync.WaitGroup
	ioStarted int32

	activeSubs map[string]map[string][]string
	subMu      sync.RWMutex

	reconnectBackoff time.Duration
	maxBackoff       time.Duration
}

func newWSHub(c *Client) *wsHub {
	wsURL := "wss://api.gateio.ws/ws/v4/"
	if c.paper {
		wsURL = "wss://ws-testnet.gate.com/v4/ws/spot"
	}
	if c.wsURLOverride != "" {
		wsURL = c.wsURLOverride
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &wsHub{
		c:                c,
		wsURL:            wsURL,
		hubCtx:           ctx,
		hubCancel:        cancel,
		activeSubs:       make(map[string]map[string][]string),
		reconnectBackoff: time.Second,
		maxBackoff:       30 * time.Second,
	}
}

func wsSign(secret, channel, event string, ts int64) string {
	msg := fmt.Sprintf("channel=%s&event=%s&time=%d", channel, event, ts)
	mac := hmac.New(sha512.New, []byte(secret))
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

func (h *wsHub) connect(ctx context.Context) error {
	h.mu.Lock()
	if h.conn != nil {
		h.mu.Unlock()
		return nil
	}
	h.mu.Unlock()

	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	if h.c.proxyURL != "" {
		if u, err := url.Parse(h.c.proxyURL); err == nil {
			dialer.Proxy = http.ProxyURL(u)
		}
	}
	conn, _, err := dialer.DialContext(ctx, h.wsURL, nil)
	if err != nil {
		return spot.NewError(spot.ErrConnection, "websocket dial", err)
	}

	h.mu.Lock()
	h.conn = conn
	h.mu.Unlock()

	if atomic.CompareAndSwapInt32(&h.ioStarted, 0, 1) {
		h.wg.Add(2)
		go h.readLoop()
		go h.heartbeat()
	}
	h.reconnectBackoff = time.Second
	return nil
}

func (h *wsHub) shutdown() error {
	h.hubCancel()
	h.wg.Wait()
	atomic.StoreInt32(&h.ioStarted, 0)

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.conn != nil {
		err := h.conn.Close()
		h.conn = nil
		return err
	}
	return nil
}

func (h *wsHub) subscribe(ctx context.Context, subs []*spot.Subscription) error {
	return h.setSubscriptions(ctx, subs, "subscribe", true)
}

func (h *wsHub) unsubscribe(ctx context.Context, subs []*spot.Subscription) error {
	return h.setSubscriptions(ctx, subs, "unsubscribe", false)
}

func (h *wsHub) setSubscriptions(ctx context.Context, subs []*spot.Subscription, event string, track bool) error {
	conn, err := h.requireConn()
	if err != nil {
		return err
	}
	for _, sub := range subs {
		if sub == nil {
			continue
		}
		ch := normalizeWSChannel(sub.Channel)
		spec, ok := wsChannelSpecFor(ch)
		if !ok {
			return spot.NewError(spot.ErrNotSupported, "unsupported channel "+ch, nil)
		}
		subCopy := *sub
		subCopy.Channel = ch
		payloads, err := spec.expand(h, &subCopy)
		if err != nil {
			return err
		}
		for _, payload := range payloads {
			if err := h.writeChannelEvent(conn, spec, event, payload); err != nil {
				return err
			}
			key := wsSubKey(ch, payload)
			h.subMu.Lock()
			if track {
				if h.activeSubs[ch] == nil {
					h.activeSubs[ch] = make(map[string][]string)
				}
				cp := append([]string(nil), payload...)
				h.activeSubs[ch][key] = cp
			} else {
				if m := h.activeSubs[ch]; m != nil {
					delete(m, key)
					if len(m) == 0 {
						delete(h.activeSubs, ch)
					}
				}
			}
			h.subMu.Unlock()
		}
	}
	_ = ctx
	return nil
}

func (h *wsHub) writeChannelEvent(conn *websocket.Conn, spec *wsChannelSpec, event string, payload []string) error {
	ts := time.Now().Unix()
	msg := map[string]interface{}{
		"time":    ts,
		"channel": spec.name,
		"event":   event,
	}
	if len(payload) > 0 {
		msg["payload"] = payload
	}
	if spec.scope == wsScopePrivate {
		msg["auth"] = map[string]string{
			"method": "api_key",
			"KEY":    h.c.apiKey,
			"SIGN":   wsSign(h.c.secret, spec.name, event, ts),
		}
	}
	if err := conn.WriteJSON(msg); err != nil {
		return spot.NewError(spot.ErrConnection, event+" "+spec.name, err)
	}
	return nil
}

func (h *wsHub) requireConn() (*websocket.Conn, error) {
	h.mu.RLock()
	conn := h.conn
	h.mu.RUnlock()
	if conn == nil {
		return nil, spot.NewError(spot.ErrNotConnected, "websocket not connected", nil)
	}
	return conn, nil
}

func (h *wsHub) readLoop() {
	defer h.wg.Done()
	for {
		select {
		case <-h.hubCtx.Done():
			return
		default:
		}
		h.mu.RLock()
		conn := h.conn
		h.mu.RUnlock()
		if conn == nil {
			time.Sleep(time.Second)
			continue
		}
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			h.c.log.ErrorContext(h.hubCtx, "ws read error", "err", err)
			if err := h.reconnectAfterError(); err != nil {
				h.c.log.ErrorContext(h.hubCtx, "ws reconnect failed", "err", err)
			}
			continue
		}
		h.dispatch(msg)
	}
}

func (h *wsHub) dispatch(msg map[string]interface{}) {
	event, _ := msg["event"].(string)
	channel, _ := msg["channel"].(string)
	switch event {
	case "subscribe", "unsubscribe":
		if errObj, ok := msg["error"].(map[string]interface{}); ok {
			h.c.log.WarnContext(h.hubCtx, "ws ctrl error",
				"event", event, "channel", channel, "error", errObj)
		} else {
			h.c.log.InfoContext(h.hubCtx, "ws ctrl", "event", event, "channel", channel)
		}
		return
	}
	if event != "update" {
		return
	}
	spec, ok := wsChannelSpecFor(channel)
	if !ok || spec.onUpdate == nil {
		return
	}
	spec.onUpdate(h, msg)
}

func (h *wsHub) heartbeat() {
	defer h.wg.Done()
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-h.hubCtx.Done():
			return
		case <-t.C:
			h.mu.RLock()
			conn := h.conn
			h.mu.RUnlock()
			if conn == nil {
				continue
			}
			ping := map[string]interface{}{
				"time":    time.Now().Unix(),
				"channel": "spot.ping",
			}
			if err := conn.WriteJSON(ping); err != nil {
				h.c.log.ErrorContext(h.hubCtx, "ws ping", "err", err)
			}
		}
	}
}

func (h *wsHub) reconnectAfterError() error {
	h.mu.Lock()
	if h.conn != nil {
		_ = h.conn.Close()
		h.conn = nil
	}
	h.mu.Unlock()

	select {
	case <-h.hubCtx.Done():
		return h.hubCtx.Err()
	case <-time.After(h.reconnectBackoff):
	}

	ctx, cancel := context.WithTimeout(h.hubCtx, 10*time.Second)
	defer cancel()
	if err := h.connect(ctx); err != nil {
		h.reconnectBackoff *= 2
		if h.reconnectBackoff > h.maxBackoff {
			h.reconnectBackoff = h.maxBackoff
		}
		return err
	}
	h.reconnectBackoff = time.Second

	h.subMu.RLock()
	snapshot := make([]struct {
		channel string
		payload []string
	}, 0)
	for ch, keys := range h.activeSubs {
		spec, ok := wsChannelSpecFor(ch)
		if !ok {
			continue
		}
		for _, payload := range keys {
			snapshot = append(snapshot, struct {
				channel string
				payload []string
			}{channel: spec.name, payload: append([]string(nil), payload...)})
		}
	}
	h.subMu.RUnlock()

	conn, err := h.requireConn()
	if err != nil {
		return err
	}
	for _, item := range snapshot {
		spec, ok := wsChannelSpecFor(item.channel)
		if !ok {
			continue
		}
		_ = h.writeChannelEvent(conn, spec, "subscribe", item.payload)
	}
	return nil
}
