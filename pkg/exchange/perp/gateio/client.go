package gateio

import (
	"context"
	"strings"
	"time"

	"github.com/kainhuck/signalix/pkg/exchange/perp"

	"github.com/antihax/optional"
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
		return perp.NewError(perp.ErrNotConnected, "REST not connected", nil)
	}
	return nil
}

func (c *Client) settleStr() string { return c.settle }

// Connect 按 parts 建立 REST / WebSocket。
func (c *Client) Connect(ctx context.Context, parts perp.ConnectParts) error {
	if parts.REST {
		if err := c.connectREST(ctx); err != nil {
			return err
		}
		c.log.InfoContext(ctx, "rest connected")
	}
	if parts.PublicWS || parts.PrivateWS {
		if parts.PrivateWS && c.userID == "" {
			return perp.NewError(perp.ErrInvalidParameter, "PrivateWS requires gateio.WithUserID", nil)
		}
		c.wsMu.Lock()
		if c.ws == nil {
			c.ws = newWSHub(c)
		}
		w := c.ws
		c.wsMu.Unlock()
		if err := w.connect(ctx); err != nil {
			return err
		}
		c.log.InfoContext(ctx, "public websocket connected")
		if parts.PrivateWS {
			if err := w.subscribe(ctx, []*perp.Subscription{
				{Channel: WSChannelOrders},
				{Channel: WSChannelUserTrades},
				{Channel: WSChannelPositions},
				{Channel: WSChannelBalances},
			}); err != nil {
				return err
			}
			c.log.InfoContext(ctx, "private websocket connected",
				"channels", []string{WSChannelOrders, WSChannelUserTrades, WSChannelPositions, WSChannelBalances})
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

// Ping 校验 REST 鉴权（需已 Connect REST）。
func (c *Client) Ping(ctx context.Context) error {
	if err := c.requireREST(); err != nil {
		return err
	}
	if err := c.waitREST(ctx); err != nil {
		return err
	}
	_, _, err := c.gate.FuturesApi.ListFuturesAccounts(c.authCtx(ctx), c.settleStr())
	return mapGateAPIError(err)
}

func (c *Client) connectREST(ctx context.Context) error {
	if err := c.waitREST(ctx); err != nil {
		return err
	}
	if _, _, err := c.gate.FuturesApi.ListFuturesAccounts(c.authCtx(ctx), c.settleStr()); err != nil {
		return mapGateAPIError(err)
	}

	var metas []*perp.ContractMeta
	offset := int32(0)
	const page = int32(100)
	for {
		if err := c.waitREST(ctx); err != nil {
			return err
		}
		contractPage, _, err := c.gate.FuturesApi.ListFuturesContracts(
			c.publicCtx(ctx), c.settleStr(), &gateapi.ListFuturesContractsOpts{
				Limit: optional.NewInt32(page), Offset: optional.NewInt32(offset),
			})
		if err != nil {
			return mapGateAPIError(err)
		}
		if len(contractPage) == 0 {
			break
		}
		for _, fc := range contractPage {
			name := strings.TrimSpace(fc.Name)
			if name == "" {
				continue
			}
			metas = append(metas, &perp.ContractMeta{
				Contract:          perp.CanonicalContract(name),
				QuantoMultiplier:  strings.TrimSpace(fc.QuantoMultiplier),
				OrderSizeMin:      strings.TrimSpace(fc.OrderSizeMin),
				OrderSizeMax:      strings.TrimSpace(fc.OrderSizeMax),
				OrderPriceStep:    decimalsInStep(fc.OrderPriceRound),
				OrderSizeStep:     decimalsInStep(fc.OrderSizeMin),
				EnableDecimalSize: fc.EnableDecimal,
			})
		}
		offset += page
		if int32(len(contractPage)) < page {
			break
		}
	}

	c.mu.Lock()
	c.restReady = true
	c.metas = metas
	c.mu.Unlock()
	return nil
}

// ListContractMeta 返回 Connect REST 后缓存的合约元数据。
func (c *Client) ListContractMeta(ctx context.Context) ([]*perp.ContractMeta, error) {
	if err := c.requireREST(); err != nil {
		return nil, err
	}
	c.mu.RLock()
	out := make([]*perp.ContractMeta, len(c.metas))
	copy(out, c.metas)
	c.mu.RUnlock()
	return out, nil
}

// Subscribe 订阅 WebSocket 频道；Channel 为空时默认为 futures.tickers（公有）。
// 私有频道需 Connect 时启用 PrivateWS 且配置 WithUserID；Contracts 为空表示 !all（适用 orders 等）。
// 复杂 payload 使用 Subscription.Payload，参见 gateio 包 WSChannel* 常量与官方文档。
func (c *Client) Subscribe(ctx context.Context, subs []*perp.Subscription) error {
	c.wsMu.Lock()
	w := c.ws
	c.wsMu.Unlock()
	if w == nil {
		return perp.NewError(perp.ErrNotConnected, "websocket not initialized, connect with PublicWS or PrivateWS", nil)
	}
	return w.subscribe(ctx, subs)
}

func (c *Client) Unsubscribe(ctx context.Context, subs []*perp.Subscription) error {
	c.wsMu.Lock()
	w := c.ws
	c.wsMu.Unlock()
	if w == nil {
		return perp.NewError(perp.ErrNotConnected, "websocket not initialized", nil)
	}
	return w.unsubscribe(ctx, subs)
}

func (c *Client) PublicEvents() <-chan *perp.PublicEvent { return c.pubCh }

func (c *Client) UserEvents() <-chan *perp.UserEvent { return c.userCh }

// Place 下单（Market / Limit）。
func (c *Client) Place(ctx context.Context, req *perp.PlaceRequest) (*perp.OrderSnapshot, error) {
	if err := c.requireREST(); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, perp.NewError(perp.ErrInvalidParameter, "request is nil", nil)
	}
	switch req.Type {
	case perp.OrderTypeMarket, perp.OrderTypeLimit:
	default:
		return nil, perp.NewError(perp.ErrNotSupported, "only Market and Limit", nil)
	}
	if req.Type == perp.OrderTypeLimit && (req.Price == nil || (strings.TrimSpace(*req.Price) == "" || parseFloat(*req.Price) <= 0)) {
		return nil, perp.NewError(perp.ErrInvalidParameter, "limit order requires positive price", nil)
	}

	contract := toGateContract(req.Contract.Canonical())
	sizeStr, err := gateOrderSizeString(req.Size, req.Side)
	if err != nil {
		return nil, err
	}

	fo := gateapi.FuturesOrder{
		Contract:   contract,
		Size:       sizeStr,
		ReduceOnly: req.ReduceOnly,
	}
	if req.Type == perp.OrderTypeMarket {
		fo.Price = "0"
		fo.Tif = "ioc"
	} else {
		fo.Price = strings.TrimSpace(*req.Price)
		fo.Tif = mapTIF(req.TimeInForce)
	}
	if cid := strings.TrimSpace(req.ClientID); cid != "" {
		fo.Text = normalizeGateOrderText(cid)
	}

	if err := c.waitREST(ctx); err != nil {
		return nil, err
	}
	out, _, err := c.gate.FuturesApi.CreateFuturesOrder(c.authCtx(ctx), c.settleStr(), fo, nil)
	if err != nil {
		return nil, mapGateAPIError(err)
	}
	return orderViewFromFutures(out), nil
}

// Cancel 撤单。
func (c *Client) Cancel(ctx context.Context, p *perp.CancelParams) error {
	if err := c.requireREST(); err != nil {
		return err
	}
	id := strings.TrimSpace(p.OrderID)
	if id == "" {
		id = normalizeGateOrderText(p.ClientOrderID)
	}
	if id == "" {
		return perp.NewError(perp.ErrInvalidParameter, "OrderID or ClientOrderID required", nil)
	}
	if err := c.waitREST(ctx); err != nil {
		return err
	}
	_, _, err := c.gate.FuturesApi.CancelFuturesOrder(c.authCtx(ctx), c.settleStr(), id, nil)
	return mapGateAPIError(err)
}

// GetOrder 按系统单号或 text 查询。
func (c *Client) GetOrder(ctx context.Context, contract perp.Contract, orderID string) (*perp.OrderSnapshot, error) {
	if err := c.requireREST(); err != nil {
		return nil, err
	}
	id := strings.TrimSpace(orderID)
	if id == "" {
		return nil, perp.NewError(perp.ErrInvalidParameter, "orderID required", nil)
	}
	if err := c.waitREST(ctx); err != nil {
		return nil, err
	}
	fo, _, err := c.gate.FuturesApi.GetFuturesOrder(c.authCtx(ctx), c.settleStr(), id)
	if err != nil {
		return nil, mapGateAPIError(err)
	}
	ov := orderViewFromFutures(fo)
	if contract != "" && ov.Contract != contract.Canonical() {
		return nil, perp.NewError(perp.ErrInvalidParameter, "order contract mismatch", nil)
	}
	return ov, nil
}

// Positions 返回非零持仓。
func (c *Client) Positions(ctx context.Context) ([]*perp.PositionSnapshot, error) {
	if err := c.requireREST(); err != nil {
		return nil, err
	}
	hold := true
	if err := c.waitREST(ctx); err != nil {
		return nil, err
	}
	list, _, err := c.gate.FuturesApi.ListPositions(c.authCtx(ctx), c.settleStr(), &gateapi.ListPositionsOpts{
		Holding: optional.NewBool(hold),
	})
	if err != nil {
		return nil, mapGateAPIError(err)
	}
	out := make([]*perp.PositionSnapshot, 0, len(list))
	for _, p := range list {
		if parseFloat(p.Size) == 0 {
			continue
		}
		pv, err := positionView(p.Contract, p)
		if err != nil {
			continue
		}
		out = append(out, pv)
	}
	return out, nil
}

// Position 单合约持仓。
func (c *Client) Position(ctx context.Context, contract perp.Contract) (*perp.PositionSnapshot, error) {
	if err := c.requireREST(); err != nil {
		return nil, err
	}
	ct := toGateContract(contract.Canonical())
	if err := c.waitREST(ctx); err != nil {
		return nil, err
	}
	pos, _, err := c.gate.FuturesApi.GetPosition(c.authCtx(ctx), c.settleStr(), ct)
	if err != nil {
		return nil, mapGateAPIError(err)
	}
	return positionView(ct, pos)
}

// Balance 结算币种账户视图。
func (c *Client) Balance(ctx context.Context) (*perp.BalanceView, error) {
	if err := c.requireREST(); err != nil {
		return nil, err
	}
	if err := c.waitREST(ctx); err != nil {
		return nil, err
	}
	ac, _, err := c.gate.FuturesApi.ListFuturesAccounts(c.authCtx(ctx), c.settleStr())
	if err != nil {
		return nil, mapGateAPIError(err)
	}
	avail := parseFloat(ac.Available)
	total := parseFloat(ac.Total)
	frozen := total - avail
	if frozen < 0 {
		frozen = 0
	}
	return &perp.BalanceView{
		Currency:  strings.ToUpper(c.settleStr()),
		Total:     trimFloatString(total),
		Available: trimFloatString(avail),
		Frozen:    trimFloatString(frozen),
		UpdatedAt: time.Now(),
	}, nil
}
