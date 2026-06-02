package models

import (
	"fmt"
	"strings"
)

// Market 交易市场。引擎层以此区分接入的市场实现（perp / spot）。
type Market string

const (
	MarketPerp Market = "perp"
	MarketSpot Market = "spot"
)

// Valid reports whether m is a known market.
func (m Market) Valid() bool {
	return m == MarketPerp || m == MarketSpot
}

// String returns the market string.
func (m Market) String() string {
	return string(m)
}

// ParseMarket parses a config / IPC market string; empty string defaults to perp
// for backward compatibility with v1 (perp-only).
func ParseMarket(s string) (Market, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return MarketPerp, nil
	}
	m := Market(s)
	if !m.Valid() {
		return "", fmt.Errorf("unknown market %q (allowed: perp, spot)", s)
	}
	return m, nil
}
