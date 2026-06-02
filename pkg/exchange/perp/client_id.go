package perp

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// GateOrderTextMaxLen Gate 永续下单 text（客户自定义 ID）最大长度。
const GateOrderTextMaxLen = 30

const gateOrderTextBodyMax = GateOrderTextMaxLen - 2 // 保留 "t-" 前缀

// NormalizeClientOrderID 转为 Gate text 字段（须 t- 前缀，总长 ≤ GateOrderTextMaxLen）。
func NormalizeClientOrderID(id string) string {
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

// GateTextFromClientOrderID 与 NormalizeClientOrderID 同义。
func GateTextFromClientOrderID(id string) string {
	return NormalizeClientOrderID(id)
}

// LocalClientIDFromGateText 从 Gate text 最佳努力还原本地 ClientID。
// 不可逆 hash 路径（28 位 hex body）返回空串，须由调用方查 Place 索引。
func LocalClientIDFromGateText(text string) string {
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
