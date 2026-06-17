package cmd

import (
	"fmt"
	"strings"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/spf13/cobra"
)

func addMarketFlag(cmd *cobra.Command, target *string) {
	cmd.Flags().StringVar(target, "market", "", "Market to query: perp or spot (default perp)")
}

func normalizeMarketFlag(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	m, err := models.ParseMarket(s)
	if err != nil {
		return "", fmt.Errorf("invalid --market: %w", err)
	}
	return m.String(), nil
}
