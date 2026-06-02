# Signalix Python SDK

用于编写 Signalix 永续策略的 Python SDK。策略由 Go 引擎以子进程方式启动，通过 **stdin/stdout 行 JSON** 与引擎通信。

## 安装

```bash
cd sdk/python
pip install -e .
```

## 策略目录结构

引擎扫描工作目录下的 `strategies/<name>/`：

```text
strategies/my_strategy/
├── config.yaml    # 引擎读取：合约、周期、预热等
└── strategy.py    # 你的 Strategy 子类
```

### config.yaml 约定

| 字段 | 说明 |
|------|------|
| `name` | 策略名（与目录名一致） |
| `enabled` | 是否随引擎启动 |
| `symbols` | 合约列表，如 `BTC/USDT` |
| `interval` | K 线周期（`1m`、`5m` 等），引擎用于 WS 订阅与 REST 预热 |
| `history_bars` | REST 预热根数，`0` 表示不预热 |
| `subscribe_ticker` | 默认省略即 `false`：引擎仍订阅 ticker 供下单定价，**不**向策略推送 `tick`；盘口策略设为 `true` 以启用 `on_tick` |
| `parameters` | **仅策略自定义参数**，经 `init` 传给 `self.parameters` |

示例：

```yaml
name: my_ma
enabled: true
symbols:
  - BTC/USDT
interval: "5m"
history_bars: 300
subscribe_ticker: false
parameters:
  ma_fast: 10
  ma_slow: 30
```

## 快速开始

### 周期策略（推荐：`on_history` + `on_kline`）

```python
import time
from signalix import Strategy, Context, KlineData, HistoryData, Signal, Direction


class MyMAStrategy(Strategy):
    def on_init(self, ctx: Context) -> None:
        self.fast = int(self.parameters.get("ma_fast", 10))
        self.slow = int(self.parameters.get("ma_slow", 30))
        self.closes: dict[str, list[float]] = {}
        ctx.info(f"init fast={self.fast} slow={self.slow}")

    def on_history(self, ctx: Context, history: HistoryData) -> None:
        """REST 预热：在 on_kline 之前调用一次"""
        for series in history.series:
            self.closes[series.contract] = [float(b.close) for b in series.bars]
        ctx.info(f"history loaded interval={history.interval}")

    def on_kline(self, ctx: Context, kline: KlineData) -> Signal | None:
        contract = kline.contract
        close = float(kline.bar.close)
        buf = self.closes.setdefault(contract, [])
        buf.append(close)
        if len(buf) > self.slow + 1:
            del buf[: len(buf) - self.slow - 1]
        if len(buf) < self.slow:
            return None

        ma_fast = sum(buf[-self.fast :]) / self.fast
        ma_slow = sum(buf[-self.slow :]) / self.slow
        prev_fast = sum(buf[-self.fast - 1 : -1]) / self.fast
        prev_slow = sum(buf[-self.slow - 1 : -1]) / self.slow

        if prev_fast <= prev_slow and ma_fast > ma_slow:
            return Signal(
                symbol=contract,
                timestamp=int(time.time()),
                direction=Direction.LONG,
                strength=0.8,
                reason="ma golden cross",
            )
        return None

    def on_tick(self, ctx: Context, tick) -> Signal | None:
        # subscribe_ticker: false 时不会收到 tick（引擎仍缓存 ticker）
        return None

    def on_stop(self, ctx: Context) -> None:
        ctx.info("strategy stopped")


if __name__ == "__main__":
    from signalix import StrategyRuntime

    StrategyRuntime(MyMAStrategy()).run()
```

本地调试可执行 `python strategy.py`；生产由 **signalixd** 拉起，无需手动运行。

## 引擎 → 策略 IPC 消息

| type | 时机 | 策略回调 |
|------|------|----------|
| `init` | 启动 | `on_init` |
| `history` | `init` 之后（`history_bars > 0`） | `on_history` |
| `kline` | 收盘 K 线 | `on_kline`，返回 `Signal` 则发单 |
| `tick` | ticker 更新（仅 `subscribe_ticker: true`） | `on_tick` |
| `stop` | 停止 | `on_stop` |

`init` 载荷：

```json
{
  "strategy_name": "my_ma",
  "symbols": ["BTC/USDT"],
  "parameters": { "ma_fast": 10 }
}
```

`kline` / `tick` 与 Go 侧对齐：`data.trace_id` + `data.kline` 或 `data.ticker`（价格为 **string**，与交易所一致）。

## Strategy 基类

| 方法 | 必须实现 | 说明 |
|------|----------|------|
| `on_init(ctx)` | 是 | 读 `self.parameters`、恢复 `ctx.get_state` |
| `on_tick(ctx, tick)` | 是* | 仅 `subscribe_ticker: true` 时有数据；可 `return None` |
| `on_stop(ctx)` | 是 | 清理、保存状态 |
| `on_kline(ctx, kline)` | 否 | 默认 `None`；周期策略在此发信号 |
| `on_history(ctx, history)` | 否 | 默认空；在此用预热 bar 初始化指标 |

\* 抽象方法要求实现 `on_tick`，无 ticker 时保持 `return None` 即可。

发信号：在 `on_kline` / `on_tick` **return `Signal`**，不要自行写 stdout。

## Context API（当前已实现）

```python
# 状态（引擎持久化）
ctx.set_state("key", {"n": 1})
n = ctx.get_state("key", default=0)

# 日志（同步到引擎）
ctx.info("message")
ctx.warn("warning")
ctx.error("error")
ctx.debug("debug")
```

以下能力通过引擎 RPC 提供（步骤五）：

```python
bal = ctx.get_balance()
pos = ctx.get_position("ETH/USDT")  # 无仓为 None
tick = ctx.get_ticker("ETH/USDT")  # 读引擎 ticker 缓存；未命中抛 RuntimeError
bars = ctx.get_klines("ETH/USDT", limit=50)  # interval 省略则用策略 config 顶层 interval
```

运行时采用 **Reader + Worker** 双线程：`on_kline` / `on_tick` / `on_history` 内可同步调用上述 API。`get_ticker` 与 `on_tick` 内 `tick.ticker` 字段相同，无需开启 `subscribe_ticker`；缓存未命中时 RPC 报错（非 `None`）。

## 数据模型

### TickData / Ticker

`subscribe_ticker: true` 时通过 `on_tick` 推送；亦可用 `ctx.get_ticker(symbol)` 主动拉取同一结构。字段与 Gate `futures.tickers` 对齐（`last`、`mark_price` 等为 string）。

### KlineData / KlineBar

每条 **收盘** K 线触发一次 `on_kline`：

```python
kline.contract      # 合约
kline.trace_id      # 追踪 ID
kline.bar.interval  # 周期
kline.bar.close     # 收盘价（string）
kline.bar.timestamp_sec
```

### HistoryData / KlineSeries

REST 预热一次性推送：

```python
history.interval     # 如 "5m"
history.series       # list[KlineSeries]
series.contract
series.bars          # list[KlineBar]，时间升序
```

### Signal

```python
# 开仓：Percent 模式须传 value（0~1，表示占用可用余额比例）
Signal(
    symbol="BTC/USDT",
    timestamp=1716200000,
    direction=Direction.LONG,
    strength=0.8,
    sizing_mode=SizingMode.PERCENT,
    value="0.1",
    reason="ma cross",
)

# 平仓：Direction.FLAT，引擎按当前持仓数量平仓，可不传 value
Signal(symbol="BTC/USDT", timestamp=1716200000, direction=Direction.FLAT, strength=1.0)

# 不传 value / sizing_mode 时，引擎用 config.toml 的 default_size_divisor 估算开仓量
Signal(symbol="BTC/USDT", timestamp=1716200000, direction=Direction.LONG, strength=0.8)
```

## 示例

| 路径 | 说明 |
|------|------|
| [example/example_trend/](example/example_trend/) | 双均线示例：`on_history` 预热 + `on_kline` 出信号 |

将 `example_trend` 复制到 `~/.signalix/strategies/example_trend/`（或你在 `config.toml` 里配置的 `strategies` 目录）后，由引擎加载。

## 测试

```bash
cd sdk/python
python3 -m unittest discover -s tests -v
```

## 注意事项

1. **周期策略**：配置 `interval` + `history_bars`，逻辑写在 `on_history` / `on_kline`，`subscribe_ticker` 保持 `false`。
2. **盘口策略**：`subscribe_ticker: true`，在 `on_tick` 中处理；注意 IPC 频率高。
3. **RPC 死锁**：`on_kline` / `on_tick` 内不要同步 `ctx.get_state` 以外的 RPC（`get_state`/`set_state` 也慎用在热路径）。
4. **崩溃保护**：5 分钟内崩溃 3 次会停止策略进程。
5. **心跳**：约每 10 秒发送；长时间阻塞回调可能触发心跳超时。
6. **价格类型**：交易所价格为 string，比较前请 `float()` 转换。

## 架构

```text
┌──────────────────┐
│  signalixd (Go)  │
│  market.Market   │
│  (perp Router)   │
│  REST 预热       │
└────────┬─────────┘
         │ stdin/stdout JSON
         │ init → history? → kline / tick?
┌────────▼─────────┐
│ StrategyRuntime  │
│  _message_loop   │
└────────┬─────────┘
         │
┌────────▼─────────┐
│  Your Strategy   │
└──────────────────┘
```

## 版本

当前版本: **0.1.0**
