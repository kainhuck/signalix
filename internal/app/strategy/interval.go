package strategy

import (
	"fmt"
	"strings"
)

// AllowedIntervals Gate 永续 K 线周期白名单。
var AllowedIntervals = []string{
	"10s", "1m", "5m", "15m", "30m", "1h", "4h", "8h", "1d", "7d",
}

var allowedIntervalSet map[string]struct{}

func init() {
	allowedIntervalSet = make(map[string]struct{}, len(AllowedIntervals))
	for _, iv := range AllowedIntervals {
		allowedIntervalSet[iv] = struct{}{}
	}
}

// ValidateInterval 校验 interval 是否在白名单内。
func ValidateInterval(interval string) error {
	interval = strings.TrimSpace(interval)
	if interval == "" {
		return fmt.Errorf("interval is required")
	}
	if _, ok := allowedIntervalSet[interval]; !ok {
		return fmt.Errorf("invalid interval %q: allowed %v", interval, AllowedIntervals)
	}
	return nil
}

// ResolveInterval 解析策略 K 线周期：顶层 interval 优先，否则 defaultInterval。
func ResolveInterval(cfg StrategyConfig, defaultInterval string) (string, error) {
	interval := strings.TrimSpace(cfg.Interval)
	if interval == "" {
		interval = strings.TrimSpace(defaultInterval)
	}
	if err := ValidateInterval(interval); err != nil {
		return "", err
	}
	return interval, nil
}
