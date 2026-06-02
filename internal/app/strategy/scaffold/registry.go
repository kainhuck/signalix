package scaffold

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"

	"github.com/kainhuck/signalix/internal/app/strategy"
	"gopkg.in/yaml.v3"
)

// TemplateMeta 内置模版元数据。
type TemplateMeta struct {
	ID              string
	Description     string
	DefaultSymbols  []string
	DefaultInterval string
}

var templateDescriptions = map[string]string{
	"trend": "Dual moving-average example (history warmup + closed klines)",
	"blank": "Minimal runnable strategy shell",
}

// ListTemplates 返回内置模版列表（按 id 排序）。
func ListTemplates() ([]TemplateMeta, error) {
	ids, err := listTemplateIDs()
	if err != nil {
		return nil, err
	}
	out := make([]TemplateMeta, 0, len(ids))
	for _, id := range ids {
		cfg, err := loadTemplateConfig(id)
		if err != nil {
			return nil, err
		}
		desc := templateDescriptions[id]
		if desc == "" {
			desc = id
		}
		out = append(out, TemplateMeta{
			ID:              id,
			Description:     desc,
			DefaultSymbols:  append([]string(nil), cfg.Symbols...),
			DefaultInterval: cfg.Interval,
		})
	}
	return out, nil
}

func listTemplateIDs() ([]string, error) {
	entries, err := fs.ReadDir(templateFS, templatesRoot)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func loadTemplateConfig(id string) (strategy.StrategyConfig, error) {
	path := filepath.Join(templatesRoot, id, strategy.StrategyConfigName)
	data, err := templateFS.ReadFile(path)
	if err != nil {
		return strategy.StrategyConfig{}, err
	}
	var cfg strategy.StrategyConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return strategy.StrategyConfig{}, fmt.Errorf("parse template %q config: %w", id, err)
	}
	return cfg, nil
}

func loadTemplateScript(id string) ([]byte, error) {
	path := filepath.Join(templatesRoot, id, strategy.StrategyScriptName)
	return templateFS.ReadFile(path)
}

func templateExists(id string) bool {
	_, err := templateFS.ReadFile(filepath.Join(templatesRoot, id, strategy.StrategyConfigName))
	return err == nil
}
