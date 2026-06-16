package gateio

import (
	"testing"

	"github.com/kainhuck/signalix/internal/ports"
)

func TestNewPerpClientReturnsPerpExchange(t *testing.T) {
	var _ ports.PerpExchange = NewPerpClient("", "")
}

func TestNewClientReturnsPerpExchange(t *testing.T) {
	var _ ports.PerpExchange = NewClient("", "")
}

func TestNewSpotClientReturnsSpotExchange(t *testing.T) {
	var _ ports.SpotExchange = NewSpotClient("", "")
}
