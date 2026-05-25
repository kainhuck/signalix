package risk

// Kind 风控判定种类。
type Kind int

const (
	KindAllow Kind = iota
	KindReject
	KindReduce
)

// Verdict 单次评估输出（无 IO）。
type Verdict struct {
	Kind         Kind
	Code         string
	Message      string
	AdjustedSize string
}

// Allow 返回允许判定。
func Allow() *Verdict {
	return &Verdict{Kind: KindAllow, Code: "RISK_ALLOW", Message: "ok"}
}
