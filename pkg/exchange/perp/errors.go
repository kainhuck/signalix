package perp

import (
	"errors"
	"fmt"
)

type Error struct {
	Code    ErrorCode
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Err }

type ErrorCode string

const (
	ErrConnection        ErrorCode = "CONNECTION_ERROR"
	ErrAuth              ErrorCode = "AUTH_ERROR"
	ErrAPI               ErrorCode = "API_ERROR"
	ErrNetwork           ErrorCode = "NETWORK_ERROR"
	ErrTimeout           ErrorCode = "TIMEOUT"
	ErrInvalidParameter  ErrorCode = "INVALID_PARAMETER"
	ErrInsufficientFunds ErrorCode = "INSUFFICIENT_FUNDS"
	ErrOrderNotFound     ErrorCode = "ORDER_NOT_FOUND"
	ErrPositionNotFound  ErrorCode = "POSITION_NOT_FOUND"
	ErrRateLimit         ErrorCode = "RATE_LIMIT"
	ErrParse             ErrorCode = "PARSE_ERROR"
	ErrUnknown           ErrorCode = "UNKNOWN"
	ErrNotConnected      ErrorCode = "NOT_CONNECTED"
	ErrNotSupported      ErrorCode = "NOT_SUPPORTED"
)

func NewError(code ErrorCode, msg string, err error) *Error {
	return &Error{Code: code, Message: msg, Err: err}
}

// IsPositionNotFound reports whether err indicates no open position for the contract
// (e.g. Gate API label POSITION_NOT_FOUND mapped by adapters).
func IsPositionNotFound(err error) bool {
	var pe *Error
	return errors.As(err, &pe) && pe.Code == ErrPositionNotFound
}

// IsOrderNotFound reports whether err indicates the order does not exist on the exchange.
func IsOrderNotFound(err error) bool {
	var pe *Error
	return errors.As(err, &pe) && pe.Code == ErrOrderNotFound
}
