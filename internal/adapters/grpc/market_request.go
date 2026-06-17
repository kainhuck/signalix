package grpc

import (
	"strings"

	"github.com/kainhuck/signalix/internal/models"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func parseMarketRequest(s string) (models.Market, error) {
	m, err := models.ParseMarket(s)
	if err != nil {
		return "", status.Error(codes.InvalidArgument, err.Error())
	}
	return m, nil
}

func isMarketNotRegisteredErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "market ") && strings.Contains(err.Error(), " not registered")
}

func protoMarket(m models.Market) string {
	if !m.Valid() {
		m = models.MarketPerp
	}
	return m.String()
}
