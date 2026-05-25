package perp

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsPositionNotFound(t *testing.T) {
	if IsPositionNotFound(nil) {
		t.Fatal("nil")
	}
	if IsPositionNotFound(errors.New("POSITION_NOT_FOUND")) {
		t.Fatal("plain string error")
	}
	if !IsPositionNotFound(NewError(ErrPositionNotFound, "no position", nil)) {
		t.Fatal("direct perp error")
	}
	wrapped := fmt.Errorf("wrap: %w", NewError(ErrPositionNotFound, "inner", nil))
	if !IsPositionNotFound(wrapped) {
		t.Fatal("wrapped perp error")
	}
}
