package gateio

import (
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/kainhuck/signalix/pkg/exchange"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"github.com/kainhuck/signalix/pkg/exchange/ratelimit"

	"github.com/gate/gateapi-go/v7"
)

// Option 配置 Gate 永续客户端。
type Option func(*Client)

// WithPaper 使用测试网 REST 与 WS。
func WithPaper(paper bool) Option {
	return func(c *Client) { c.paper = paper }
}

// WithSettle 结算币种，默认 usdt。
func WithSettle(settle string) Option {
	return func(c *Client) {
		if settle != "" {
			c.settle = strings.ToLower(settle)
		}
	}
}

// WithUserID Gate 私有频道 futures.orders 所需用户数字 ID。
func WithUserID(userID string) Option {
	return func(c *Client) { c.userID = strings.TrimSpace(userID) }
}

// WithLogger 注入日志。
func WithLogger(log exchange.Logger) Option {
	return func(c *Client) {
		if log != nil {
			c.log = log
		}
	}
}

// WithRESTBasePath 覆盖 REST 根路径；模拟盘默认见 NewClient（paper=true 时的 BasePath）。
func WithRESTBasePath(base string) Option {
	return func(c *Client) {
		c.restBaseOverride = strings.TrimSpace(base)
	}
}

// WithRateLimit sets a global REST rate limit (requests per second); <=0 disables.
func WithRateLimit(requestsPerSecond int) Option {
	return func(c *Client) {
		c.restLimiter = ratelimit.New(requestsPerSecond)
	}
}

// WithProxy 设置 HTTP 代理地址，同时作用于 REST 与 WebSocket。
func WithProxy(proxyURL string) Option {
	return func(c *Client) {
		c.proxyURL = strings.TrimSpace(proxyURL)
	}
}

// WithChannelBuffers 设置公共/私有 WS 事件通道容量。
func WithChannelBuffers(publicBuf, privateBuf int) Option {
	return func(c *Client) {
		if publicBuf > 0 {
			c.pubCh = make(chan *perp.PublicEvent, publicBuf)
		}
		if privateBuf > 0 {
			c.userCh = make(chan *perp.UserEvent, privateBuf)
		}
	}
}

// Client 实现 perp.Live（Gate USDT 本位永续）。
type Client struct {
	apiKey, secret   string
	paper            bool
	settle           string
	userID           string
	restBaseOverride string
	proxyURL         string

	log exchange.Logger

	gate *gateapi.APIClient

	restLimiter *ratelimit.Limiter

	mu        sync.RWMutex
	restReady bool
	metas     []*perp.ContractMeta

	wsMu sync.Mutex
	ws   *wsHub

	pubCh  chan *perp.PublicEvent
	userCh chan *perp.UserEvent
}

var _ perp.Live = (*Client)(nil)

// NewClient 创建客户端；调用 Connect 后根据 ConnectParts 建立链路。
func NewClient(apiKey, secret string, opts ...Option) *Client {
	apiKey = strings.TrimSpace(apiKey)
	secret = strings.TrimSpace(secret)
	c := &Client{
		apiKey: apiKey,
		secret: secret,
		settle: "usdt",
		log:    exchange.NopLogger,
		pubCh:  make(chan *perp.PublicEvent, 512),
		userCh: make(chan *perp.UserEvent, 256),
	}
	cfg := gateapi.NewConfiguration()
	cfg.Key = apiKey
	cfg.Secret = secret
	c.gate = gateapi.NewAPIClient(cfg)
	for _, o := range opts {
		o(c)
	}
	if c.proxyURL != "" {
		if u, err := url.Parse(c.proxyURL); err == nil {
			c.gate.GetConfig().HTTPClient = &http.Client{
				Transport: &http.Transport{
					Proxy: http.ProxyURL(u),
				},
			}
		}
	}
	if c.restBaseOverride != "" {
		c.gate.ChangeBasePath(c.restBaseOverride)
	} else if c.paper {
		c.gate.ChangeBasePath("https://api-testnet.gateapi.io/api/v4")
	}
	return c
}

// APIClient 返回底层 SDK（高级用法）。
func (c *Client) APIClient() *gateapi.APIClient {
	return c.gate
}
