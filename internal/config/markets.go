package config

import (
	"fmt"
	"strings"

	"github.com/kainhuck/signalix/internal/models"
)

// EnabledMarkets 返回解析后的启用市场列表（缺省为 perp）。
func (c *Config) EnabledMarkets() ([]models.Market, error) {
	enabled := []string{"perp"}
	if c != nil && len(c.Markets.Enabled) > 0 {
		enabled = c.Markets.Enabled
	}
	seen := make(map[models.Market]bool)
	var out []models.Market
	for _, raw := range enabled {
		s := strings.TrimSpace(strings.ToLower(raw))
		if s == "" {
			continue
		}
		m, err := models.ParseMarket(s)
		if err != nil {
			return nil, fmt.Errorf("markets.enabled: %w", err)
		}
		if m == models.MarketSpot {
			return nil, fmt.Errorf("market %q not supported in this build", m)
		}
		if !m.Valid() {
			return nil, fmt.Errorf("markets.enabled: invalid market %q", raw)
		}
		if seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}
	if len(out) == 0 {
		return []models.Market{models.MarketPerp}, nil
	}
	return out, nil
}
