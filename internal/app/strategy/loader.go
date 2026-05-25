package strategy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"github.com/kainhuck/signalix/pkg/logger"
	"gopkg.in/yaml.v3"
)

var (
	StrategyConfigName = "config.yaml"
	StrategyScriptName = "strategy.py"
)

// StrategyConfig 策略配置 对应策略目录下的 config.yaml
type StrategyConfig struct {
	Name            string                 `yaml:"name"`             // 策略名称
	Enabled         bool                   `yaml:"enabled"`          // 是否启用
	Symbols         []perp.Contract        `yaml:"symbols"`          // 策略运行市场
	Interval        string                 `yaml:"interval"`         // K 线周期（引擎订阅）
	HistoryBars     int                    `yaml:"history_bars"`     // REST 预热根数；0 表示不预热
	SubscribeTicker bool                   `yaml:"subscribe_ticker"` // 默认 false：引擎仍订 ticker 供定价，不向策略发 tick
	Parameters      map[string]interface{} `yaml:"parameters"`       // 策略自定义参数
}

// Strategy 策略对象
type Strategy struct {
	StrategyConfig

	ScriptPath string // 策略脚本完整路径
}

// StrategyLoader 策略加载器
type StrategyLoader struct {
	ctx           context.Context
	StrategiesDir string // 策略存放目录，目前只支持单目录 后续可拓展成多个目录
}

// NewStrategyLoader 创建策略加载器
func NewStrategyLoader(ctx context.Context, strategiesDir string) *StrategyLoader {
	return &StrategyLoader{
		ctx:           ctx,
		StrategiesDir: strategiesDir,
	}
}

// DiscoverStrategies 发现所有策略
func (sl *StrategyLoader) DiscoverStrategies() (map[string]*Strategy, error) {
	strategies := make(map[string]*Strategy)

	// 检查策略目录是否存在
	if _, err := os.Stat(sl.StrategiesDir); os.IsNotExist(err) {
		return strategies, fmt.Errorf("strategies directory not found: %s", sl.StrategiesDir)
	}

	// 遍历策略目录
	entries, err := os.ReadDir(sl.StrategiesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read strategies directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		strategyDir := filepath.Join(sl.StrategiesDir, entry.Name())
		scriptPath := filepath.Join(strategyDir, StrategyScriptName)
		configPath := filepath.Join(strategyDir, StrategyConfigName)

		// 检查策略脚本是否存在
		if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
			continue
		}

		// 检查配置文件是否存在
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			continue
		}

		// 加载策略配置
		config, err := sl.LoadStrategyConfig(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load strategy %s: %w", entry.Name(), err)
		}

		if _, ok := strategies[config.Name]; ok {
			logger.WarnContext(sl.ctx, "strategy duplicate naming", logger.String("strategy", config.Name))
		}

		strategies[config.Name] = &Strategy{
			StrategyConfig: config,
			ScriptPath:     scriptPath,
		}
	}

	return strategies, nil
}

// LoadStrategyConfig 加载策略配置
func (sl *StrategyLoader) LoadStrategyConfig(configPath string) (StrategyConfig, error) {
	var config StrategyConfig

	// 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return config, fmt.Errorf("failed to read config file: %w", err)
	}

	// 解析 YAML
	if err := yaml.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("failed to parse config file: %w", err)
	}

	// 验证配置
	if err := sl.validateConfig(config); err != nil {
		return config, fmt.Errorf("invalid config: %w", err)
	}

	return config, nil
}

// validateConfig 验证策略配置
func (sl *StrategyLoader) validateConfig(config StrategyConfig) error {
	if config.Name == "" {
		return fmt.Errorf("strategy name is required")
	}

	if len(config.Symbols) == 0 {
		return fmt.Errorf("at least one symbol is required")
	}

	// 验证 symbols 格式
	for _, symbol := range config.Symbols {
		if symbol == "" {
			return fmt.Errorf("empty symbol in symbols list")
		}
	}

	if iv := strings.TrimSpace(config.Interval); iv != "" {
		if err := ValidateInterval(iv); err != nil {
			return err
		}
	}

	if config.HistoryBars < 0 {
		return fmt.Errorf("history_bars must be non-negative")
	}
	if config.HistoryBars > 2000 {
		return fmt.Errorf("history_bars must not exceed 2000")
	}

	return nil
}
