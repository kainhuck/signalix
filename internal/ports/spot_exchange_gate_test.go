package ports_test

import (
	"testing"

	"github.com/kainhuck/signalix/internal/ports"
	"github.com/kainhuck/signalix/pkg/exchange/spot/gateio"
)

func TestGateIOSpotClientImplementsSpotExchange(t *testing.T) {
	var _ ports.SpotExchange = (*gateio.Client)(nil)
}
