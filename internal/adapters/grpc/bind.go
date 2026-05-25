package grpc

import (
	"fmt"
	"net"
	"strings"
)

// VerifyLoopbackListenAddr 拒绝非本机监听，除非 insecureBindAll 为 true（config grpc.insecure_bind_all）。
func VerifyLoopbackListenAddr(addr string, insecureBindAll bool) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid grpc listen addr %q: %w", addr, err)
	}
	if insecureBindAll {
		return nil
	}
	h := strings.TrimSpace(host)
	if h == "" || h == "0.0.0.0" {
		return fmt.Errorf("refusing bind on %q without grpc.insecure_bind_all=true", host)
	}
	if h != "127.0.0.1" && h != "localhost" && h != "::1" {
		return fmt.Errorf("refusing non-loopback grpc host %q (set grpc.insecure_bind_all=true to override)", h)
	}
	return nil
}
