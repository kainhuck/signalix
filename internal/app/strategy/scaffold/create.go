package scaffold

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kainhuck/signalix/internal/app/strategy"
	"gopkg.in/yaml.v3"
)

var (
	// ErrTemplateNotFound 表示未知模版 ID。
	ErrTemplateNotFound = errors.New("template not found")
	// ErrAlreadyExists 表示目标策略目录已存在。
	ErrAlreadyExists = errors.New("strategy directory already exists")
	// ErrInvalidConfig 表示合并后配置校验失败。
	ErrInvalidConfig = errors.New("invalid strategy config")
)

// CreateOptions 创建策略目录的参数。
type CreateOptions struct {
	StrategiesDir   string
	Name            string
	TemplateID      string
	Symbols         []string
	Interval        *string
	HistoryBars     *int
	SubscribeTicker *bool
	Enabled         *bool
	Parameters      map[string]interface{}
}

// Create 在 strategiesDir 下生成策略目录，返回绝对路径。
func Create(opts CreateOptions) (string, error) {
	if err := ValidateName(opts.Name); err != nil {
		return "", err
	}
	if opts.TemplateID == "" {
		return "", fmt.Errorf("template_id is required")
	}
	if !templateExists(opts.TemplateID) {
		return "", fmt.Errorf("%w: %s", ErrTemplateNotFound, opts.TemplateID)
	}
	if opts.StrategiesDir == "" {
		return "", fmt.Errorf("strategies dir is required")
	}

	base, err := loadTemplateConfig(opts.TemplateID)
	if err != nil {
		return "", err
	}
	cfg := mergeConfig(base, opts)
	if err := strategy.ValidateConfig(cfg); err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}

	script, err := loadTemplateScript(opts.TemplateID)
	if err != nil {
		return "", err
	}

	dir := filepath.Join(opts.StrategiesDir, opts.Name)
	if _, err := os.Stat(dir); err == nil {
		return "", fmt.Errorf("%w: %s", ErrAlreadyExists, opts.Name)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.Mkdir(dir, 0o755); err != nil {
		if os.IsExist(err) {
			return "", fmt.Errorf("%w: %s", ErrAlreadyExists, opts.Name)
		}
		return "", err
	}

	rollback := true
	defer func() {
		if rollback {
			_ = os.RemoveAll(dir)
		}
	}()

	configPath := filepath.Join(dir, strategy.StrategyConfigName)
	configData, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(configPath, configData, 0o644); err != nil {
		return "", err
	}
	scriptPath := filepath.Join(dir, strategy.StrategyScriptName)
	if err := os.WriteFile(scriptPath, script, 0o644); err != nil {
		return "", err
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	rollback = false
	return abs, nil
}

func mergeConfig(base strategy.StrategyConfig, opts CreateOptions) strategy.StrategyConfig {
	cfg := base
	cfg.Name = opts.Name
	if opts.Enabled != nil {
		cfg.Enabled = *opts.Enabled
	} else {
		cfg.Enabled = false
	}
	if len(opts.Symbols) > 0 {
		cfg.Symbols = append([]string(nil), opts.Symbols...)
	}
	if opts.Interval != nil {
		cfg.Interval = *opts.Interval
	}
	if opts.HistoryBars != nil {
		cfg.HistoryBars = *opts.HistoryBars
	}
	if opts.SubscribeTicker != nil {
		cfg.SubscribeTicker = *opts.SubscribeTicker
	}
	if len(opts.Parameters) > 0 {
		if cfg.Parameters == nil {
			cfg.Parameters = make(map[string]interface{})
		}
		for k, v := range opts.Parameters {
			cfg.Parameters[k] = v
		}
	}
	return cfg
}
