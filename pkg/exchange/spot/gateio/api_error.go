package gateio

import (
	"fmt"

	"github.com/kainhuck/signalix/pkg/exchange/spot"

	"github.com/gate/gateapi-go/v7"
)

func mapGateAPIError(err error) error {
	if err == nil {
		return nil
	}
	ge, ok := err.(gateapi.GateAPIError)
	if !ok {
		return spot.NewError(spot.ErrUnknown, err.Error(), err)
	}
	msg := ge.GetMessage()
	if msg == "" {
		msg = ge.Error()
	}
	switch ge.Label {
	case "INVALID_KEY", "INVALID_SIGNATURE", "MISSING_REQUIRED_HEADER", "INVALID_CREDENTIALS":
		return spot.NewError(spot.ErrAuth, msg, nil)
	case "ORDER_NOT_FOUND":
		return spot.NewError(spot.ErrOrderNotFound, msg, nil)
	case "INSUFFICIENT_BALANCE":
		return spot.NewError(spot.ErrInsufficientFunds, msg, nil)
	case "TOO_MANY_REQUESTS", "REQUEST_LIMIT":
		return spot.NewError(spot.ErrRateLimit, msg, nil)
	default:
		return spot.NewError(spot.ErrAPI, fmt.Sprintf("%s (%s)", msg, ge.Label), nil)
	}
}
