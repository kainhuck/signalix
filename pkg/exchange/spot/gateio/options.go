package gateio

import (
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/kainhuck/signalix/pkg/exchange"
	"github.com/kainhuck/signalix/pkg/exchange/ratelimit"
	"github.com/kainhuck/signalix/pkg/exchange/spot"

	"github.com/gate/gateapi-go/v7"
)

// Option 配置 Gate 现货客户端。
type Option func(*Client)

// WithPaper 使用测试网 REST（与永续相同 BasePath）与 WS。
func WithPaper(paper bool) Option {
	return func(c *Client) { c.paper = paper }
}

// WithLogger 注入日志。
func WithLogger(log exchange.Logger) Option {
	return func(c *Client) {
		if log != nil {
			c.log = log
		}
	}
}

// WithRESTBasePath 覆盖 REST 根路径。
func WithRESTBasePath(base string) Option {
	return func(c *Client) {
		c.restBaseOverride = strings.TrimSpace(base)
	}
}

// WithWSURL 覆盖 WebSocket 根 URL。
func WithWSURL(url string) Option {
	return func(c *Client) { c.wsURLOverride = strings.TrimSpace(url) }
}

// WithProxy 设置 HTTP 代理地址，同时作用于 REST 与 WebSocket。
func WithProxy(proxyURL string) Option {
	return func(c *Client) {
		c.proxyURL = strings.TrimSpace(proxyURL)
	}
}

// WithRateLimit sets a global REST rate limit (requests per second); <=0 disables.
func WithRateLimit(requestsPerSecond int) Option {
	return func(c *Client) {
		c.restLimiter = ratelimit.New(requestsPerSecond)
	}
}

// WithChannelBuffers 设置公共/私有 WS 事件通道容量。
func WithChannelBuffers(publicBuf, privateBuf int) Option {
	return func(c *Client) {
		if publicBuf > 0 {
			c.pubCh = make(chan *spot.PublicEvent, publicBuf)
		}
		if privateBuf > 0 {
			c.userCh = make(chan *spot.UserEvent, privateBuf)
		}
	}
}

// Client 实现 spot.Live（Gate 现货）。
type Client struct {
	apiKey, secret   string
	paper            bool
	restBaseOverride string
	wsURLOverride    string
	proxyURL         string

	log exchange.Logger

	gate *gateapi.APIClient

	restLimiter *ratelimit.Limiter

	mu        sync.RWMutex
	restReady bool
	metas     []*spot.PairMeta

	wsMu sync.Mutex
	ws   *wsHub

	pubCh  chan *spot.PublicEvent
	userCh chan *spot.UserEvent
}

var _ spot.Live = (*Client)(nil)

// NewClient 创建现货客户端；调用 Connect 后根据 ConnectParts 建立链路。
func NewClient(apiKey, secret string, opts ...Option) *Client {
	apiKey = strings.TrimSpace(apiKey)
	secret = strings.TrimSpace(secret)
	c := &Client{
		apiKey: apiKey,
		secret: secret,
		log:    exchange.NopLogger,
		pubCh:  make(chan *spot.PublicEvent, 512),
		userCh: make(chan *spot.UserEvent, 256),
	}
	cfg := gateapi.NewConfiguration()
	cfg.Key = apiKey
	cfg.Secret = secret
	c.gate = gateapi.NewAPIClient(cfg)
	for _, o := range opts {
		o(c)
	}
	if c.restBaseOverride != "" {
		c.gate.ChangeBasePath(c.restBaseOverride)
	} else if c.paper {
		c.gate.ChangeBasePath("https://api-testnet.gateapi.io/api/v4")
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
	return c
}

// APIClient 返回底层 SDK（高级用法）。
func (c *Client) APIClient() *gateapi.APIClient {
	return c.gate
}
