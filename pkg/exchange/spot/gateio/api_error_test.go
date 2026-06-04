package gateio

import (
	"errors"
	"testing"

	"github.com/gate/gateapi-go/v7"
	"github.com/kainhuck/signalix/pkg/exchange/spot"
)

func TestMapGateAPIError_auth(t *testing.T) {
	err := mapGateAPIError(gateapi.GateAPIError{Label: "INVALID_KEY", Message: "bad key"})
	var se *spot.Error
	if !errors.As(err, &se) || se.Code != spot.ErrAuth {
		t.Fatalf("got %v", err)
	}
}

func TestMapGateAPIError_orderNotFound(t *testing.T) {
	err := mapGateAPIError(gateapi.GateAPIError{Label: "ORDER_NOT_FOUND", Message: "missing"})
	var se *spot.Error
	if !errors.As(err, &se) || se.Code != spot.ErrOrderNotFound {
		t.Fatalf("got %v", err)
	}
}

func TestMapGateAPIError_rateLimit(t *testing.T) {
	err := mapGateAPIError(gateapi.GateAPIError{Label: "TOO_MANY_REQUESTS", Message: "slow down"})
	var se *spot.Error
	if !errors.As(err, &se) || se.Code != spot.ErrRateLimit {
		t.Fatalf("got %v", err)
	}
}
