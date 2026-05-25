package models

import "errors"

// Validation errors
var (
	ErrInvalidSymbol     = errors.New("invalid symbol")
	ErrInvalidPrice      = errors.New("invalid price")
	ErrInvalidBidAsk     = errors.New("bid price must be less than ask price")
	ErrInvalidVolume     = errors.New("invalid volume")
	ErrInvalidDirection  = errors.New("invalid direction")
	ErrInvalidStrength   = errors.New("strength must be between 0 and 1")
	ErrInvalidSizingMode = errors.New("invalid sizing mode")
	ErrInvalidValue      = errors.New("invalid value")
	ErrInvalidOrderSide  = errors.New("invalid order side")
	ErrInvalidSize       = errors.New("invalid size")
	ErrInvalidStopPrice  = errors.New("invalid stop price")
)
