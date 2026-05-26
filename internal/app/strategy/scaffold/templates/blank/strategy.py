"""Minimal strategy scaffold — extend on_kline / on_init as needed."""
from __future__ import annotations

from typing import Optional

from signalix import Strategy, Context, KlineData


class BlankStrategy(Strategy):
    def on_init(self, ctx: Context) -> None:
        ctx.info("BlankStrategy init")

    def on_kline(self, ctx: Context, kline: KlineData) -> Optional[None]:
        return None


if __name__ == "__main__":
    from signalix import StrategyRuntime

    runtime = StrategyRuntime(BlankStrategy())
    runtime.run()
