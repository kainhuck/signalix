"""
Signalix 数据模型

定义策略开发中使用的所有数据结构
"""

from dataclasses import dataclass, field
from enum import Enum
from typing import Optional, Dict, Any, List
from datetime import datetime

from signalix import strategy


def _symbol(data: Dict[str, Any]) -> str:
    return data.get("contract") or data.get("symbol", "")


@dataclass
class Ticker:
    """ticker数据（contract 为兼容字段，symbol 为统一别名）"""
    contract: str
    last: str
    mark_price: str
    index_price: str
    funding_rate: str
    change_pct_24h: str
    volume_24h: str
    volume_24h_base: str
    volume_24h_quote: str
    open_interest: str
    low_24h: str
    high_24h: str
    timestamp_millis: int

    @property
    def symbol(self) -> str:
        return self.contract

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Ticker":
        """从字典创建"""
        return cls(
            contract=_symbol(data),
            last=data["last"],
            mark_price=data["mark_price"],
            index_price=data["index_price"],
            funding_rate=data["funding_rate"],
            change_pct_24h=data["change_pct_24h"],
            volume_24h=data["volume_24h"],
            volume_24h_base=data["volume_24h_base"],
            volume_24h_quote=data["volume_24h_quote"],
            open_interest=data["open_interest"],
            low_24h=data["low_24h"],
            high_24h=data["high_24h"],
            timestamp_millis=int(data["timestamp_millis"]),
        )

@dataclass
class TickData:
    """市场数据"""
    contract: str
    trace_id: str
    ticker: "Ticker"

    @property
    def symbol(self) -> str:
        return self.contract

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "TickData":
        """从字典创建（与 IPC tick：trace_id + ticker 对齐）"""
        ticker = Ticker.from_dict(data["ticker"])
        return cls(
            contract=ticker.contract,
            trace_id=data.get("trace_id", ""),
            ticker=ticker,
        )


@dataclass
class KlineBar:
    """收盘 K 线（contract 为兼容字段，symbol 为统一别名）"""
    contract: str
    interval: str
    open: str
    high: str
    low: str
    close: str
    volume: str
    volume_base: str
    timestamp_sec: int
    window_closed: bool

    @property
    def symbol(self) -> str:
        return self.contract

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "KlineBar":
        return cls(
            contract=_symbol(data),
            interval=data["interval"],
            open=data["open"],
            high=data["high"],
            low=data["low"],
            close=data["close"],
            volume=data.get("volume", ""),
            volume_base=data.get("volume_base", ""),
            timestamp_sec=int(data["timestamp_sec"]),
            window_closed=bool(data.get("window_closed", True)),
        )


@dataclass
class KlineData:
    """引擎推送的 K 线事件"""
    contract: str
    trace_id: str
    bar: KlineBar

    @property
    def symbol(self) -> str:
        return self.contract

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "KlineData":
        bar = KlineBar.from_dict(data["kline"])
        return cls(
            contract=bar.contract,
            trace_id=data.get("trace_id", ""),
            bar=bar,
        )


@dataclass
class KlineSeries:
    """单合约历史 K 线序列"""
    contract: str
    bars: list[KlineBar]

    @property
    def symbol(self) -> str:
        return self.contract

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "KlineSeries":
        return cls(
            contract=_symbol(data),
            bars=[KlineBar.from_dict(b) for b in data.get("bars", [])],
        )


@dataclass
class BalanceData:
    """账户余额（get_balance RPC）"""
    currency: str
    total: str
    available: str
    frozen: str
    updated_at: int

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "BalanceData":
        return cls(
            currency=data.get("currency", ""),
            total=data.get("total", ""),
            available=data.get("available", ""),
            frozen=data.get("frozen", ""),
            updated_at=int(data.get("updated_at", 0)),
        )


@dataclass
class PositionData:
    """持仓快照（contract 为兼容字段，symbol 为统一别名）"""
    contract: str
    side: str
    size: str
    entry_price: str
    mark_price: str
    unrealized_pnl: str
    leverage: int
    updated_at: int

    @property
    def symbol(self) -> str:
        return self.contract

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "PositionData":
        return cls(
            contract=_symbol(data),
            side=data.get("side", ""),
            size=data.get("size", ""),
            entry_price=data.get("entry_price", ""),
            mark_price=data.get("mark_price", ""),
            unrealized_pnl=data.get("unrealized_pnl", ""),
            leverage=int(data.get("leverage", 0)),
            updated_at=int(data.get("updated_at", 0)),
        )


@dataclass
class HistoryData:
    """REST 预热历史 K 线"""
    interval: str
    series: list[KlineSeries]

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "HistoryData":
        return cls(
            interval=data["interval"],
            series=[KlineSeries.from_dict(s) for s in data.get("series", [])],
        )


class Direction(str, Enum):
    """交易方向"""
    LONG = "Long"
    SHORT = "Short"
    FLAT = "Flat"


class SizingMode(str, Enum):
    """仓位计算模式"""
    PERCENT = "Percent"  # 账户百分比
    FIXED = "Fixed"  # 固定数量
    CUSTOM = "Custom"  # 风险百分比


@dataclass
class Signal:
    """交易信号"""
    symbol: str
    timestamp: int
    direction: Direction
    strength: float  # 0.0 ~ 1.0
    price: Optional[str] = None
    sizing_mode: SizingMode = SizingMode.PERCENT
    value: Optional[str] = None
    reason: str = ""

    def to_dict(self) -> Dict[str, Any]:
        """转换为字典（与 Go decision 引擎约定对齐）"""
        out: Dict[str, Any] = {
            "symbol": self.symbol,
            "direction": self.direction.value,
            "strength": self.strength,
            "reason": self.reason,
            "timestamp": self.timestamp,
        }
        if self.price is not None:
            out["price"] = self.price
        # 未指定 value 时不传 sizing_mode，引擎按可用余额/default_size_divisor 估算数量
        if self.value is not None:
            out["sizing_mode"] = self.sizing_mode.value
            out["value"] = self.value
        return out
