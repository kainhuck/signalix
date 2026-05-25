package gateio

import (
	"fmt"

	"github.com/kainhuck/signalix/pkg/exchange/perp"

	"github.com/gate/gateapi-go/v7"
)

func mapGateAPIError(err error) error {
	if err == nil {
		return nil
	}
	ge, ok := err.(gateapi.GateAPIError)
	if !ok {
		return perp.NewError(perp.ErrUnknown, err.Error(), err)
	}
	msg := ge.GetMessage()
	if msg == "" {
		msg = ge.Error()
	}
	switch ge.Label {
	case "INVALID_KEY", "INVALID_SIGNATURE", "MISSING_REQUIRED_HEADER", "INVALID_CREDENTIALS":
		return perp.NewError(perp.ErrAuth, msg, nil)
	case "ORDER_NOT_FOUND":
		return perp.NewError(perp.ErrOrderNotFound, msg, nil)
	case "POSITION_NOT_FOUND":
		return perp.NewError(perp.ErrPositionNotFound, msg, nil)
	case "INSUFFICIENT_BALANCE":
		return perp.NewError(perp.ErrInsufficientFunds, msg, nil)
	case "TOO_MANY_REQUESTS", "REQUEST_LIMIT":
		return perp.NewError(perp.ErrRateLimit, msg, nil)
	default:
		return perp.NewError(perp.ErrAPI, fmt.Sprintf("%s (%s)", msg, ge.Label), nil)
	}
}
