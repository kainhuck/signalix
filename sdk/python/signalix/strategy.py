"""
Signalix 策略基类

定义策略的抽象接口
"""

from abc import ABC, abstractmethod
from typing import TYPE_CHECKING, Optional

if TYPE_CHECKING:
    from .context import Context
    from .models import TickData, KlineData, HistoryData, Signal


class Strategy(ABC):
    """
    策略基类

    所有策略必须继承此类并实现三个生命周期方法：
    - on_init(): 策略初始化
    - on_tick(): 处理市场数据更新
    - on_stop(): 策略停止清理
    """

    def __init__(self):
        """初始化策略"""
        self.name: str = ""
        self.symbols: list[str] = []
        self.parameters: dict = {}

    @abstractmethod
    def on_init(self, ctx: "Context") -> None:
        """
        策略初始化回调

        在策略启动时调用一次，用于：
        - 加载历史数据
        - 初始化指标
        - 恢复策略状态

        Args:
            ctx: 上下文对象，提供数据查询和状态管理接口
        """
        pass

    @abstractmethod
    def on_tick(self, ctx: "Context", tick: "TickData") -> Optional["Signal"]:
        """
        ticker 更新回调（需策略配置 subscribe_ticker 时才有推送，见步骤三）。

        返回 Signal 则发单；返回 None 表示不下单。
        """
        pass

    def on_kline(self, ctx: "Context", kline: "KlineData") -> Optional["Signal"]:
        """
        收盘 K 线回调（周期策略主路径）。

        默认不交易；子类可覆盖并 return Signal。
        注意：勿在本回调内同步 rpc_call，会阻塞消息循环。
        """
        return None

    def on_history(self, ctx: "Context", history: "HistoryData") -> None:
        """
        REST 预热历史 K 线；默认空实现。在此初始化指标，勿发 Signal。
        勿在回调内同步 rpc_call。
        """
        return None

    @abstractmethod
    def on_stop(self, ctx: "Context") -> None:
        """
        策略停止回调

        在策略停止前调用一次，用于：
        - 保存策略状态
        - 清理资源
        - 记录日志

        Args:
            ctx: 上下文对象
        """
        pass
