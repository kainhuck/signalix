package ports_test

import (
	"testing"

	"github.com/kainhuck/signalix/internal/ports"
	"github.com/kainhuck/signalix/pkg/exchange/perp/gateio"
)

func TestGateIOClientImplementsExchange(t *testing.T) {
	var _ ports.PerpExchange = (*gateio.Client)(nil)
}
