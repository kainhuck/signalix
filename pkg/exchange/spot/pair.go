package spot

import "strings"

// Pair 现货交易对标识，规范写法为 BASE/QUOTE，例如 BTC/USDT。
type Pair string

// CanonicalPair 将任意常见写法归一为 BASE/QUOTE。
func CanonicalPair(s string) Pair {
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
	return Pair(strings.Join(parts, "/"))
}

func slashFromCompactUSDT(s string) string {
	u := strings.ToUpper(strings.TrimSpace(s))
	if strings.HasSuffix(u, "USDT") && len(u) > 4 {
		return strings.TrimSuffix(u, "USDT") + "/USDT"
	}
	return u
}

// String 返回规范表示。
func (p Pair) String() string { return string(p) }

// Canonical 再次规范化。
func (p Pair) Canonical() Pair {
	return CanonicalPair(string(p))
}

// BaseCurrency returns the base asset of the pair (e.g. BTC for BTC/USDT).
func (p Pair) BaseCurrency() string {
	parts := strings.Split(string(p.Canonical()), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

// QuoteCurrency returns the quote asset of the pair (e.g. USDT for BTC/USDT).
func (p Pair) QuoteCurrency() string {
	parts := strings.Split(string(p.Canonical()), "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}
