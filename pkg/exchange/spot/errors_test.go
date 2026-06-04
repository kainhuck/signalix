package spot

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsOrderNotFound(t *testing.T) {
	if IsOrderNotFound(nil) {
		t.Fatal("nil")
	}
	if IsOrderNotFound(errors.New("ORDER_NOT_FOUND")) {
		t.Fatal("plain string error")
	}
	if !IsOrderNotFound(NewError(ErrOrderNotFound, "no order", nil)) {
		t.Fatal("direct spot error")
	}
	wrapped := fmt.Errorf("wrap: %w", NewError(ErrOrderNotFound, "inner", nil))
	if !IsOrderNotFound(wrapped) {
		t.Fatal("wrapped spot error")
	}
}
