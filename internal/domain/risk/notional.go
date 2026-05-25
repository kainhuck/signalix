package risk

import "github.com/shopspring/decimal"

// ContractNotionalUSDT 计算 |size| × markPrice × quantoMultiplier。
func ContractNotionalUSDT(size, markPrice, quantoMultiplier decimal.Decimal) decimal.Decimal {
	if size.Sign() == 0 || markPrice.Sign() <= 0 || quantoMultiplier.Sign() <= 0 {
		return decimal.Zero
	}
	return size.Abs().Mul(markPrice).Mul(quantoMultiplier)
}

// LeverageRatio 返回 totalExposure / equity；equity <= 0 时返回零。
func LeverageRatio(totalExposure, equity decimal.Decimal) decimal.Decimal {
	if equity.Sign() <= 0 {
		return decimal.Zero
	}
	return totalExposure.Div(equity)
}
