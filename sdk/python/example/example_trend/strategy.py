"""
示例策略：双均线（收盘 K 线驱动）

- 引擎 REST 预热 → on_history
- WebSocket 收盘 K 线 → on_kline
- subscribe_ticker: false，不向策略推送 tick（引擎仍订 ticker 供下单定价）
"""
from __future__ import annotations

import time
from typing import Optional

from signalix import Strategy, Context, KlineData, HistoryData, Signal, Direction, SizingMode, TickData


class TrendStrategy(Strategy):
    """简单双均线示例：金叉做多、死叉平多（仅演示，非实盘建议）"""

    def on_init(self, ctx: Context) -> None:
        self.ma_fast = int(self.parameters.get("ma_fast", 10))
        self.ma_slow = int(self.parameters.get("ma_slow", 30))
        if self.ma_fast >= self.ma_slow:
            raise ValueError("ma_fast must be less than ma_slow")
        self.closes: dict[str, list[float]] = {}
        # 开仓占用可用余额比例（Go Percent 模式要求 0~1 的 value）
        self.position_pct = str(self.parameters.get("position_pct", "0.1"))
        ctx.info(f"TrendStrategy init ma_fast={self.ma_fast} ma_slow={self.ma_slow}")

    def on_history(self, ctx: Context, history: HistoryData) -> None:
        for series in history.series:
            self.closes[series.contract] = [float(b.close) for b in series.bars]
            ctx.info(
                f"history {series.contract} bars={len(series.bars)} "
                f"interval={history.interval}"
            )

    def on_kline(self, ctx: Context, kline: KlineData) -> Optional[Signal]:
        contract = kline.contract
        close = float(kline.bar.close)
        buf = self.closes.setdefault(contract, [])
        buf.append(close)

        max_len = self.ma_slow + 1
        if len(buf) > max_len:
            del buf[: len(buf) - max_len]
        if len(buf) < self.ma_slow:
            return None

        ma_fast = sum(buf[-self.ma_fast :]) / self.ma_fast
        ma_slow = sum(buf[-self.ma_slow :]) / self.ma_slow
        prev_fast = sum(buf[-self.ma_fast - 1 : -1]) / self.ma_fast
        prev_slow = sum(buf[-self.ma_slow - 1 : -1]) / self.ma_slow

        ctx.debug(
            f"kline {contract} close={close} ma_fast={ma_fast:.4f} ma_slow={ma_slow:.4f}"
        )

        # 金叉：快线从下穿上慢线
        if prev_fast <= prev_slow and ma_fast > ma_slow:
            ctx.info(f"golden cross {contract}")
            return Signal(
                symbol=contract,
                timestamp=int(time.time()),
                direction=Direction.LONG,
                strength=0.8,
                sizing_mode=SizingMode.PERCENT,
                value=self.position_pct,
                reason="ma golden cross",
            )

        # 死叉：快线从上穿下慢线 → 平仓
        if prev_fast >= prev_slow and ma_fast < ma_slow:
            ctx.info(f"death cross {contract}")
            return Signal(
                symbol=contract,
                timestamp=int(time.time()),
                direction=Direction.FLAT,
                strength=1.0,
                reason="ma death cross",
            )

        return None

    def on_tick(self, ctx: Context, tick: TickData) -> Optional[Signal]:
        # config.yaml 中 subscribe_ticker: false，正常不会进入此回调
        return None

    def on_stop(self, ctx: Context) -> None:
        ctx.info("TrendStrategy stopped")


if __name__ == "__main__":
    from signalix import StrategyRuntime

    runtime = StrategyRuntime(TrendStrategy())
    runtime.run()
