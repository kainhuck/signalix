package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kainhuck/signalix/internal/domain/risk"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)

const (
	EnvWorkDir       = "SIGNALIX_WORK_DIR"
	DefaultWorkDir   = "~/.signalix"
	ConfigFileName   = "config.toml"
	DatabaseRelPath  = "data/signalix.db"
	StrategiesRelDir = "strategies"
)

// Config 全局配置（$SIGNALIX_WORK_DIR/config.toml）。
type Config struct {
	WorkDir       string
	ConfigPath    string
	DatabasePath  string
	StrategiesDir string
	Strategies    StrategiesConfig

	Log      LogConfig
	Exchange ExchangeConfig
	Risk     RiskConfig
	Database DatabaseConfig
	Channels ChannelsConfig
	Decision DecisionConfig
	GRPC     GRPCConfig

	projectionRefresh time.Duration
}

type LogConfig struct {
	LogLevel   string `mapstructure:"log_level"`
	File       string `mapstructure:"file"`
	MaxSizeMB  int    `mapstructure:"max_size_mb"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAgeDays int    `mapstructure:"max_age_days"`
	Compress   bool   `mapstructure:"compress"`
}

type StrategiesConfig struct {
	Dir             string `mapstructure:"dir"`
	DefaultInterval string `mapstructure:"default_interval"`
}

type ExchangeConfig struct {
	Name            string          `mapstructure:"name"`
	APIKey          string          `mapstructure:"api_key"`
	APISecret       string          `mapstructure:"api_secret"`
	Paper           bool            `mapstructure:"paper"`
	UserID          string          `mapstructure:"user_id"`
	Settle          string          `mapstructure:"settle"`
	RESTBasePath    string          `mapstructure:"rest_base_path"`
	RateLimit       int             `mapstructure:"rate_limit"`
	MaxRetries      int             `mapstructure:"max_retries"`
	Connect         ExchangeConnect `mapstructure:"connect"`
	PublicWSBuffer  int             `mapstructure:"public_ws_buffer"`
	PrivateWSBuffer int             `mapstructure:"private_ws_buffer"`
}

type ExchangeConnect struct {
	REST      bool `mapstructure:"rest"`
	PublicWS  bool `mapstructure:"public_ws"`
	PrivateWS bool `mapstructure:"private_ws"`
}

type RiskConfig struct {
	EnableRiskControl  bool    `mapstructure:"enable_risk_control"`
	MaxOrderSize       float64 `mapstructure:"max_order_size"`
	MaxOpenOrders      int     `mapstructure:"max_open_orders"`
	MaxPositions       int     `mapstructure:"max_positions"`
	MaxPositionSize    float64 `mapstructure:"max_position_size"`
	MaxDailyLoss       float64 `mapstructure:"max_daily_loss"`
	MaxDrawdown        float64 `mapstructure:"max_drawdown"`
	MaxLeverage        int     `mapstructure:"max_leverage"`
	EnablePositionLock bool    `mapstructure:"enable_position_lock"`
}

type DatabaseConfig struct {
	MaxOpenConns int `mapstructure:"max_open_conns"`
}

type ChannelsConfig struct {
	Signal         int `mapstructure:"signal"`
	Order          int `mapstructure:"order"`
	MainOrder      int `mapstructure:"main_order"`
	Market         int `mapstructure:"market"`
	OMSOrderUpdate int `mapstructure:"oms_order_update"`
	OMSCmd         int `mapstructure:"oms_cmd"`
}

type DecisionConfig struct {
	DefaultSizeDivisor int `mapstructure:"default_size_divisor"`
}

type GRPCConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	Addr            string `mapstructure:"addr"`
	Token           string `mapstructure:"token"`
	InsecureBindAll bool   `mapstructure:"insecure_bind_all"`
}

// Load 解析工作目录与 config.toml，填充 Config（缺失字段使用默认值）。
func Load() (*Config, error) {
	workDir, err := ResolveWorkDir()
	if err != nil {
		return nil, err
	}
	configPath := filepath.Join(workDir, ConfigFileName)

	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("toml")
	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read config %s: %w", configPath, err)
		}
	}

	var raw struct {
		Log        LogConfig        `mapstructure:"log"`
		Strategies StrategiesConfig `mapstructure:"strategies"`
		Exchange   ExchangeConfig   `mapstructure:"exchange"`
		Risk       RiskConfig       `mapstructure:"risk"`
		Database   DatabaseConfig   `mapstructure:"database"`
		Channels   ChannelsConfig   `mapstructure:"channels"`
		Projection struct {
			RefreshInterval string `mapstructure:"refresh_interval"`
		} `mapstructure:"projection"`
		Decision DecisionConfig `mapstructure:"decision"`
		GRPC     GRPCConfig     `mapstructure:"grpc"`
	}
	if err := v.Unmarshal(&raw); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	strategiesDir, err := resolvePath(workDir, raw.Strategies.Dir)
	if err != nil {
		return nil, err
	}

	logFile := strings.TrimSpace(raw.Log.File)
	if logFile != "" {
		logFile, err = resolvePath(workDir, logFile)
		if err != nil {
			return nil, err
		}
	}

	refresh, err := time.ParseDuration(strings.TrimSpace(raw.Projection.RefreshInterval))
	if err != nil {
		return nil, fmt.Errorf("projection.refresh_interval: %w", err)
	}

	cfg := &Config{
		WorkDir:           workDir,
		ConfigPath:        configPath,
		DatabasePath:      filepath.Join(workDir, DatabaseRelPath),
		StrategiesDir:     strategiesDir,
		Strategies:        raw.Strategies,
		Log:               raw.Log,
		Exchange:          raw.Exchange,
		Risk:              raw.Risk,
		Database:          raw.Database,
		Channels:          raw.Channels,
		Decision:          raw.Decision,
		GRPC:              normalizeGRPC(raw.GRPC),
		projectionRefresh: refresh,
	}
	cfg.Log.File = logFile

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// ProjectionRefreshInterval 返回解析后的刷新周期。
func (c *Config) ProjectionRefreshInterval() time.Duration {
	if c == nil || c.projectionRefresh <= 0 {
		return 10 * time.Second
	}
	return c.projectionRefresh
}

func (c *Config) validate() error {
	name := strings.ToLower(strings.TrimSpace(c.Exchange.Name))
	if name == "" {
		c.Exchange.Name = "gateio"
		name = "gateio"
	}
	if name != "gateio" {
		return fmt.Errorf("unsupported exchange.name %q (only gateio)", c.Exchange.Name)
	}
	if strings.TrimSpace(c.Exchange.APIKey) == "" || strings.TrimSpace(c.Exchange.APISecret) == "" {
		return fmt.Errorf("exchange.api_key and exchange.api_secret are required in %s", c.ConfigPath)
	}
	if c.Exchange.Connect.PrivateWS && strings.TrimSpace(c.Exchange.UserID) == "" {
		return fmt.Errorf("exchange.user_id is required when exchange.connect.private_ws is true")
	}
	if c.Decision.DefaultSizeDivisor <= 0 {
		return fmt.Errorf("decision.default_size_divisor must be positive")
	}
	return nil
}

// RiskRules 将 risk 段转为 domain 规则（仅已实现的字段生效）。
func (c *Config) RiskRules() risk.Rules {
	if c == nil {
		return risk.DefaultRules()
	}
	def := risk.DefaultRules()
	out := def
	r := c.Risk
	out.Enable = r.EnableRiskControl
	if r.MaxOrderSize > 0 {
		out.MaxOrderSize = decimal.NewFromFloat(r.MaxOrderSize)
	}
	if r.MaxOpenOrders > 0 {
		out.MaxOpenOrders = r.MaxOpenOrders
	}
	if r.MaxPositions > 0 {
		out.MaxPositions = r.MaxPositions
	}
	return out
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("log.log_level", "info")
	v.SetDefault("log.max_size_mb", 100)
	v.SetDefault("log.max_backups", 5)
	v.SetDefault("log.max_age_days", 30)
	v.SetDefault("log.compress", true)

	v.SetDefault("strategies.dir", StrategiesRelDir)

	v.SetDefault("exchange.name", "gateio")
	v.SetDefault("exchange.paper", true)
	v.SetDefault("exchange.settle", "usdt")
	v.SetDefault("exchange.rate_limit", 10)
	v.SetDefault("exchange.max_retries", 3)
	v.SetDefault("exchange.connect.rest", true)
	v.SetDefault("exchange.connect.public_ws", true)
	v.SetDefault("exchange.connect.private_ws", true)
	v.SetDefault("exchange.public_ws_buffer", 512)
	v.SetDefault("exchange.private_ws_buffer", 256)

	v.SetDefault("risk.enable_risk_control", true)
	v.SetDefault("risk.max_order_size", 100000.0)
	v.SetDefault("risk.max_open_orders", 50)
	v.SetDefault("risk.max_positions", 20)

	v.SetDefault("database.max_open_conns", 1)

	v.SetDefault("channels.signal", 100)
	v.SetDefault("channels.order", 100)
	v.SetDefault("channels.main_order", 100)
	v.SetDefault("channels.market", 1000)
	v.SetDefault("channels.oms_order_update", 100)
	v.SetDefault("channels.oms_cmd", 100)

	v.SetDefault("projection.refresh_interval", "10s")
	v.SetDefault("decision.default_size_divisor", 10)

	v.SetDefault("grpc.enabled", false)
	v.SetDefault("grpc.addr", "127.0.0.1:50051")
	v.SetDefault("grpc.insecure_bind_all", false)
}

func normalizeGRPC(g GRPCConfig) GRPCConfig {
	g.Addr = strings.TrimSpace(g.Addr)
	if g.Addr != "" && !strings.Contains(g.Addr, ":") {
		g.Addr += ":50051"
	}
	return g
}

// ResolveWorkDir 读取 SIGNALIX_WORK_DIR，默认 ~/.signalix。
func ResolveWorkDir() (string, error) {
	raw := strings.TrimSpace(os.Getenv(EnvWorkDir))
	if raw == "" {
		raw = DefaultWorkDir
	}
	return expandPath(raw)
}

func resolvePath(base, p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p), nil
	}
	return filepath.Clean(filepath.Join(base, p)), nil
}

func expandPath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = filepath.Join(home, p[2:])
	}
	return filepath.Clean(p), nil
}
