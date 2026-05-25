package grpc

import (
	"testing"
)

func TestVerifyLoopbackListenAddr(t *testing.T) {
	t.Run("loopback_ok", func(t *testing.T) {
		if err := VerifyLoopbackListenAddr("127.0.0.1:50051", false); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("zero_rejected", func(t *testing.T) {
		if err := VerifyLoopbackListenAddr("0.0.0.0:50051", false); err == nil {
			t.Fatal("expected error for 0.0.0.0")
		}
	})
	t.Run("zero_allowed_insecure", func(t *testing.T) {
		if err := VerifyLoopbackListenAddr("0.0.0.0:50051", true); err != nil {
			t.Fatal(err)
		}
	})
}
