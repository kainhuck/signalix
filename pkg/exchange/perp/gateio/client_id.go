package gateio

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const gateOrderTextMaxLen = 30

const gateOrderTextBodyMax = gateOrderTextMaxLen - 2 // 保留 "t-" 前缀

// TagFromLocal 满足 perp.ClientOrderIDCodec（Gate text）。
func (c *Client) TagFromLocal(localID string) string {
	return normalizeGateOrderText(localID)
}

// LocalFromTag 满足 perp.ClientOrderIDCodec。
func (c *Client) LocalFromTag(tag string) (string, bool) {
	local := localIDFromGateText(tag)
	return local, local != ""
}

func normalizeGateOrderText(id string) string {
	raw := strings.TrimSpace(id)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "t-") {
		raw = strings.TrimPrefix(raw, "t-")
	}
	if len(raw) > gateOrderTextBodyMax {
		sum := sha256.Sum256([]byte(raw))
		raw = hex.EncodeToString(sum[:])[:gateOrderTextBodyMax]
	}
	return "t-" + raw
}

func localIDFromGateText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" || !strings.HasPrefix(text, "t-") {
		return ""
	}
	body := strings.TrimPrefix(text, "t-")
	if len(body) == gateOrderTextBodyMax && isHexString(body) {
		return ""
	}
	return body
}

func isHexString(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			continue
		}
		return false
	}
	return len(s) > 0
}
