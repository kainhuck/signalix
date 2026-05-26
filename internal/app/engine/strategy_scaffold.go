package engine

import (
	"errors"
	"fmt"

	"github.com/kainhuck/signalix/internal/app/strategy/scaffold"
)

// ErrStrategyLoaderNotConfigured 表示策略 loader 未初始化。
var ErrStrategyLoaderNotConfigured = errors.New("strategy loader not configured")

// ListStrategyTemplates 返回内置策略模版元数据。
func (e *Engine) ListStrategyTemplates() ([]scaffold.TemplateMeta, error) {
	return scaffold.ListTemplates()
}

// CreateStrategyScaffold 按模版创建策略目录并刷新 catalog。
func (e *Engine) CreateStrategyScaffold(opts scaffold.CreateOptions) (path string, catalogCount int, err error) {
	if e == nil || e.loader == nil {
		return "", 0, ErrStrategyLoaderNotConfigured
	}
	opts.StrategiesDir = e.loader.StrategiesDir
	path, err = scaffold.Create(opts)
	if err != nil {
		return "", 0, err
	}
	n, err := e.ReloadStrategiesCatalog()
	if err != nil {
		return path, 0, fmt.Errorf("reload catalog: %w", err)
	}
	return path, n, nil
}
