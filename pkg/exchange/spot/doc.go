// Package spot 定义现货交易的领域类型与细粒度接口，
// 供各交易所适配器实现。量化策略应依赖本包接口而非具体交易所包。
//
// 交易对 Pair 的规范写法为 BASE/QUOTE（例如 BTC/USDT）；适配层负责与
// 各所原生符号（如 Gate 的 BASE_QUOTE）互转。
package spot
