"""
Signalix 策略上下文

提供策略运行时的数据查询和状态管理接口
"""

import logging
from typing import Optional, Any, Dict, List, TYPE_CHECKING

if TYPE_CHECKING:
    from .runtime import StrategyRuntime

from .models import BalanceData, PositionData, KlineBar


class Context:
    """
    策略上下文

    提供以下功能：
    - 数据查询：持仓、余额、市场数据、K线
    - 状态管理：保存和读取策略状态
    - 信号发送：发送交易信号
    - 日志记录：记录策略日志
    """

    def __init__(self, runtime: "StrategyRuntime"):
        """
        初始化上下文

        Args:
            runtime: 策略运行时对象
        """
        self._runtime = runtime
        self._logger = logging.getLogger(f"signalix.strategy.{runtime.strategy_name}")

    # ==================== 状态管理 ====================

    def get_state(self, key: str, default: Any = None) -> Any:
        """
        读取策略状态

        Args:
            key: 状态键
            default: 默认值

        Returns:
            状态值，如果不存在返回 default
        """
        result = self._runtime.rpc_call("get_state", {"key": key})
        if result is None:
            return default
        return result

    def get_balance(self) -> Optional[BalanceData]:
        """查询账户余额（引擎 AccountProjection）。"""
        result = self._runtime.rpc_call("get_balance", {})
        if result is None:
            return None
        return BalanceData.from_dict(result)

    def get_position(self, symbol: str) -> Optional[PositionData]:
        """查询指定合约持仓；无仓返回 None。"""
        result = self._runtime.rpc_call("get_position", {"symbol": symbol})
        if result is None:
            return None
        return PositionData.from_dict(result)

    def get_klines(
            self,
            symbol: str,
            interval: Optional[str] = None,
            limit: int = 100,
    ) -> List[KlineBar]:
        """查询引擎缓冲的收盘 K 线（升序）。interval 省略时使用策略 config 顶层周期。"""
        params: Dict[str, Any] = {"symbol": symbol, "limit": limit}
        if interval is not None:
            params["interval"] = interval
        result = self._runtime.rpc_call("get_klines", params)
        if not result:
            return []
        return [KlineBar.from_dict(b) for b in result]

    def set_state(self, key: str, value: Any) -> None:
        """
        保存策略状态

        Args:
            key: 状态键
            value: 状态值（必须可 JSON 序列化）
        """
        self._runtime.rpc_call("set_state", {"key": key, "value": value})

    # ==================== 日志记录 ====================

    def debug(self, message: str) -> None:
        """记录 DEBUG 日志"""
        self._logger.debug(message)
        self._runtime.send_log("DEBUG", message)

    def info(self, message: str) -> None:
        """记录 INFO 日志"""
        self._logger.info(message)
        self._runtime.send_log("INFO", message)

    def warn(self, message: str) -> None:
        """记录 WARN 日志"""
        self._logger.warning(message)
        self._runtime.send_log("WARN", message)

    def error(self, message: str) -> None:
        """记录 ERROR 日志"""
        self._logger.error(message)
        self._runtime.send_log("ERROR", message)
