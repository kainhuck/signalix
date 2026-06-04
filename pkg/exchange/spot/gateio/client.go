package gateio

import (
	"context"
	"strings"

	"github.com/kainhuck/signalix/pkg/exchange/spot"

	"github.com/gate/gateapi-go/v7"
)

func (c *Client) authCtx(parent context.Context) context.Context {
	return context.WithValue(parent, gateapi.ContextGateAPIV4, gateapi.GateAPIV4{
		Key:    c.apiKey,
		Secret: c.secret,
	})
}

func (c *Client) publicCtx(parent context.Context) context.Context {
	return context.WithValue(parent, gateapi.ContextPublic, true)
}

func (c *Client) requireREST() error {
	c.mu.RLock()
	ok := c.restReady
	c.mu.RUnlock()
	if !ok {
		return spot.NewError(spot.ErrNotConnected, "REST not connected", nil)
	}
	return nil
}

// Connect 按 parts 建立 REST / WebSocket。
func (c *Client) Connect(ctx context.Context, parts spot.ConnectParts) error {
	if parts.REST {
		if err := c.connectREST(ctx); err != nil {
			return err
		}
		c.log.InfoContext(ctx, "spot rest connected")
	}
	if parts.PublicWS || parts.PrivateWS {
		c.wsMu.Lock()
		if c.ws == nil {
			c.ws = newWSHub(c)
		}
		w := c.ws
		c.wsMu.Unlock()
		if err := w.connect(ctx); err != nil {
			return err
		}
		c.log.InfoContext(ctx, "spot websocket connected")
		if parts.PrivateWS {
			if err := w.subscribe(ctx, []*spot.Subscription{
				{Channel: WSChannelBalances},
			}); err != nil {
				return err
			}
			c.log.InfoContext(ctx, "spot private websocket connected",
				"channels", []string{WSChannelBalances})
		}
	}
	return nil
}

// Close 释放 WebSocket 与 REST 就绪状态。
func (c *Client) Close(ctx context.Context) error {
	c.wsMu.Lock()
	if c.ws != nil {
		_ = c.ws.shutdown()
		c.ws = nil
	}
	c.wsMu.Unlock()

	c.mu.Lock()
	c.restReady = false
	c.metas = nil
	c.mu.Unlock()
	return nil
}

// Subscribe 订阅 WebSocket 频道；Channel 为空时默认为 spot.tickers。
func (c *Client) Subscribe(ctx context.Context, subs []*spot.Subscription) error {
	c.wsMu.Lock()
	w := c.ws
	c.wsMu.Unlock()
	if w == nil {
		return spot.NewError(spot.ErrNotConnected, "websocket not initialized, connect with PublicWS or PrivateWS", nil)
	}
	return w.subscribe(ctx, subs)
}

// Unsubscribe 取消 WebSocket 订阅。
func (c *Client) Unsubscribe(ctx context.Context, subs []*spot.Subscription) error {
	c.wsMu.Lock()
	w := c.ws
	c.wsMu.Unlock()
	if w == nil {
		return spot.NewError(spot.ErrNotConnected, "websocket not initialized", nil)
	}
	return w.unsubscribe(ctx, subs)
}

// PublicEvents 返回公共 WS 事件通道。
func (c *Client) PublicEvents() <-chan *spot.PublicEvent { return c.pubCh }

// UserEvents 返回私有 WS 事件通道。
func (c *Client) UserEvents() <-chan *spot.UserEvent { return c.userCh }

// Ping 校验 REST 鉴权（需已 Connect REST）。
func (c *Client) Ping(ctx context.Context) error {
	if err := c.requireREST(); err != nil {
		return err
	}
	if err := c.waitREST(ctx); err != nil {
		return err
	}
	_, _, err := c.gate.SpotApi.ListSpotAccounts(c.authCtx(ctx), nil)
	return mapGateAPIError(err)
}

func (c *Client) connectREST(ctx context.Context) error {
	if err := c.waitREST(ctx); err != nil {
		return err
	}
	if _, _, err := c.gate.SpotApi.ListSpotAccounts(c.authCtx(ctx), nil); err != nil {
		return mapGateAPIError(err)
	}

	if err := c.waitREST(ctx); err != nil {
		return err
	}
	pairs, _, err := c.gate.SpotApi.ListCurrencyPairs(c.publicCtx(ctx))
	if err != nil {
		return mapGateAPIError(err)
	}

	metas := make([]*spot.PairMeta, 0, len(pairs))
	for _, cp := range pairs {
		if pm := pairMetaFromGate(cp); pm != nil {
			metas = append(metas, pm)
		}
	}

	c.mu.Lock()
	c.restReady = true
	c.metas = metas
	c.mu.Unlock()
	return nil
}

// ListPairMeta 返回 Connect REST 后缓存的交易对元数据。
func (c *Client) ListPairMeta(ctx context.Context) ([]*spot.PairMeta, error) {
	if err := c.requireREST(); err != nil {
		return nil, err
	}
	c.mu.RLock()
	out := make([]*spot.PairMeta, len(c.metas))
	copy(out, c.metas)
	c.mu.RUnlock()
	return out, nil
}

// Place 下单（Market / Limit）。
func (c *Client) Place(ctx context.Context, req *spot.PlaceRequest) (*spot.OrderSnapshot, error) {
	if err := c.requireREST(); err != nil {
		return nil, err
	}
	ord, err := buildGateOrder(req)
	if err != nil {
		return nil, err
	}
	if err := c.waitREST(ctx); err != nil {
		return nil, err
	}
	out, _, err := c.gate.SpotApi.CreateOrder(c.authCtx(ctx), ord, nil)
	if err != nil {
		return nil, mapGateAPIError(err)
	}
	return orderViewFromSpot(out), nil
}

// Cancel 撤单。
func (c *Client) Cancel(ctx context.Context, p *spot.CancelParams) error {
	if err := c.requireREST(); err != nil {
		return err
	}
	if p == nil {
		return spot.NewError(spot.ErrInvalidParameter, "cancel params is nil", nil)
	}
	id := strings.TrimSpace(p.OrderID)
	if id == "" {
		id = normalizeClientOrderID(p.ClientOrderID)
	}
	if id == "" {
		return spot.NewError(spot.ErrInvalidParameter, "OrderID or ClientOrderID required", nil)
	}
	pair := toGatePair(p.Pair)
	if pair == "" {
		return spot.NewError(spot.ErrInvalidParameter, "pair is required", nil)
	}
	if err := c.waitREST(ctx); err != nil {
		return err
	}
	_, _, err := c.gate.SpotApi.CancelOrder(c.authCtx(ctx), id, pair, nil)
	return mapGateAPIError(err)
}

// GetOrder 按系统单号或 text 查询。
func (c *Client) GetOrder(ctx context.Context, pair spot.Pair, orderID string) (*spot.OrderSnapshot, error) {
	if err := c.requireREST(); err != nil {
		return nil, err
	}
	id := strings.TrimSpace(orderID)
	if id == "" {
		return nil, spot.NewError(spot.ErrInvalidParameter, "orderID required", nil)
	}
	gp := toGatePair(pair)
	if gp == "" {
		return nil, spot.NewError(spot.ErrInvalidParameter, "pair is required", nil)
	}
	if err := c.waitREST(ctx); err != nil {
		return nil, err
	}
	o, _, err := c.gate.SpotApi.GetOrder(c.authCtx(ctx), id, gp, nil)
	if err != nil {
		return nil, mapGateAPIError(err)
	}
	ov := orderViewFromSpot(o)
	if pair != "" && ov.Pair != pair.Canonical() {
		return nil, spot.NewError(spot.ErrInvalidParameter, "order pair mismatch", nil)
	}
	return ov, nil
}

// Balances 返回现货账户各币种余额（跳过 total 为 0 的条目）。
func (c *Client) Balances(ctx context.Context) ([]*spot.BalanceView, error) {
	if err := c.requireREST(); err != nil {
		return nil, err
	}
	if err := c.waitREST(ctx); err != nil {
		return nil, err
	}
	list, _, err := c.gate.SpotApi.ListSpotAccounts(c.authCtx(ctx), nil)
	if err != nil {
		return nil, mapGateAPIError(err)
	}
	out := make([]*spot.BalanceView, 0, len(list))
	for _, ac := range list {
		bv := balanceViewFromSpot(ac)
		if parseFloat(bv.Total) == 0 {
			continue
		}
		out = append(out, bv)
	}
	return out, nil
}

// Balance 返回单币种余额；不存在时返回零余额。
func (c *Client) Balance(ctx context.Context, currency string) (*spot.BalanceView, error) {
	cur := strings.ToUpper(strings.TrimSpace(currency))
	if cur == "" {
		return nil, spot.NewError(spot.ErrInvalidParameter, "currency is required", nil)
	}
	all, err := c.Balances(ctx)
	if err != nil {
		return nil, err
	}
	for _, bv := range all {
		if bv.Currency == cur {
			return bv, nil
		}
	}
	return &spot.BalanceView{
		Currency:  cur,
		Total:     "0",
		Available: "0",
		Frozen:    "0",
	}, nil
}
