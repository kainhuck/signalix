package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// Signal 策略信号
type Signal struct {
	Symbol     string      `json:"symbol"`
	Direction  Direction   `json:"direction"`
	Strength   float64     `json:"strength"`
	Price      *string     `json:"price,omitempty"`
	SizingMode *SizingMode `json:"sizing_mode,omitempty"`
	Value      *string     `json:"value,omitempty"`
	Reason     string      `json:"reason,omitempty"`
	Timestamp  int64       `json:"timestamp"`
}

type Direction string

const (
	DirectionLong  Direction = "Long"
	DirectionShort Direction = "Short"
	DirectionFlat  Direction = "Flat"
)

type SizingMode string

const (
	SizingModePercent SizingMode = "Percent"
	SizingModeFixed   SizingMode = "Fixed"
	SizingModeCustom  SizingMode = "Custom"
)

// StrategySignal 策略信号
type StrategySignal struct {
	StrategyName string
	Signal       *Signal
	Timestamp    time.Time
}

// Validate validates Signal
func (s *Signal) Validate() error {
	if s.Symbol == "" {
		return ErrInvalidSymbol
	}
	if s.Direction != DirectionLong && s.Direction != DirectionShort && s.Direction != DirectionFlat {
		return ErrInvalidDirection
	}
	if s.Strength < 0 || s.Strength > 1 {
		return ErrInvalidStrength
	}
	if s.Price != nil {
		price, _ := decimal.NewFromString(*s.Price)
		if price.LessThanOrEqual(decimal.Zero) {
			return ErrInvalidPrice
		}
	}
	if s.SizingMode != nil {
		if *s.SizingMode != SizingModePercent && *s.SizingMode != SizingModeFixed && *s.SizingMode != SizingModeCustom {
			return ErrInvalidSizingMode
		}
	}
	if s.Value != nil {
		value, _ := decimal.NewFromString(*s.Value)
		if value.LessThanOrEqual(decimal.Zero) {
			return ErrInvalidValue
		}
	}
	return nil
}
