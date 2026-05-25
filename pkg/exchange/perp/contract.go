package perp

import "strings"

// CanonicalContract 将任意常见写法归一为 BASE/QUOTE（如 BTC/USDT）。
// 接受：已含斜杠的形式、Gate 下划线形式、紧凑写法（如 BTCUSDT 且报价为 USDT）。
func CanonicalContract(s string) Contract {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "_", "/")
	if !strings.Contains(s, "/") {
		s = slashFromCompactUSDT(s)
	}
	parts := strings.Split(s, "/")
	for i := range parts {
		parts[i] = strings.ToUpper(strings.TrimSpace(parts[i]))
	}
	return Contract(strings.Join(parts, "/"))
}

func slashFromCompactUSDT(s string) string {
	u := strings.ToUpper(strings.TrimSpace(s))
	if strings.HasSuffix(u, "USDT") && len(u) > 4 {
		return strings.TrimSuffix(u, "USDT") + "/USDT"
	}
	return u
}

// String 返回规范表示。
func (c Contract) String() string { return string(c) }

// Canonical 再次规范化（用于外部传入可能未归一的字符串）。
func (c Contract) Canonical() Contract {
	return CanonicalContract(string(c))
}
