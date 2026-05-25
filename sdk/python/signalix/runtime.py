"""
Signalix 策略运行时

处理 IPC 通信、消息循环和心跳监控
"""

import sys
import json
import time
import threading
import logging
from typing import Any, Dict, Optional
from queue import Queue, Empty

from .strategy import Strategy
from .context import Context
from .models import TickData, KlineData, HistoryData, Signal


class StrategyRuntime:
    """
    策略运行时

    负责：
    - IPC 消息收发（JSON over stdin/stdout）
    - Reader 线程读 stdin + Worker 线程处理（避免 on_kline 内 RPC 死锁）
    - RPC 请求处理
    - 心跳发送
    - 崩溃检测和恢复
    """

    def __init__(self, strategy: Strategy):
        self.strategy = strategy
        self.strategy_name = ""
        self.running = False

        self._inbound_queue: Queue = Queue()
        self._stdout_lock = threading.Lock()

        # RPC 管理
        self._rpc_counter = 0
        self._rpc_pending: Dict[str, Queue] = {}
        self._rpc_lock = threading.Lock()

        self._reader_thread: Optional[threading.Thread] = None
        self._worker_thread: Optional[threading.Thread] = None
        self._heartbeat_thread: Optional[threading.Thread] = None
        self._heartbeat_interval = 10

        self._crash_times: list[float] = []
        self._crash_window = 300
        self._crash_limit = 3

        self._logger = logging.getLogger("signalix.runtime")
        logging.basicConfig(
            level=logging.INFO,
            format="%(asctime)s [%(levelname)s] %(message)s",
            stream=sys.stderr,
        )

    def run(self) -> None:
        """启动策略运行时，阻塞直到 stop。"""
        self.running = True

        try:
            self._start_heartbeat()
            self._reader_thread = threading.Thread(
                target=self._reader_loop, name="signalix-reader", daemon=True
            )
            self._worker_thread = threading.Thread(
                target=self._worker_loop, name="signalix-worker", daemon=False
            )
            self._reader_thread.start()
            self._worker_thread.start()
            self._worker_thread.join()
        except KeyboardInterrupt:
            self._logger.info("Received interrupt signal")
        except Exception as e:
            self._logger.error(f"Runtime error: {e}", exc_info=True)
            self._record_crash()
        finally:
            self.running = False
            self._stop_heartbeat()

    def _reader_loop(self) -> None:
        """仅读取 stdin 并入队，不执行策略回调。"""
        try:
            for line in sys.stdin:
                if not self.running:
                    break
                line = line.strip()
                if not line:
                    continue
                try:
                    msg = json.loads(line)
                except json.JSONDecodeError as e:
                    self._logger.error(f"Failed to parse message: {e}")
                    continue
                self._inbound_queue.put(msg)
                if msg.get("type") == "stop":
                    break
        except Exception as e:
            self._logger.error(f"Reader loop error: {e}", exc_info=True)
        finally:
            self._inbound_queue.put(None)

    def _worker_loop(self) -> None:
        """从队列取消息并分发（可同步 rpc_call）。"""
        while self.running:
            try:
                msg = self._inbound_queue.get(timeout=0.5)
            except Empty:
                continue
            if msg is None:
                break
            try:
                self._handle_message(msg)
                if msg.get("type") == "stop":
                    self.running = False
                    break
            except Exception as e:
                self._logger.error(f"Failed to handle message: {e}", exc_info=True)

    def _handle_message(self, msg: Dict[str, Any]) -> None:
        msg_type = msg.get("type")
        data = msg.get("data", {})

        if msg_type == "init":
            self._handle_init(data)
        elif msg_type == "tick":
            self._handle_tick(data)
        elif msg_type == "kline":
            self._handle_kline(data)
        elif msg_type == "history":
            self._handle_history(data)
        elif msg_type == "stop":
            self._handle_stop(data)
        elif msg_type == "rpc_response":
            self._handle_rpc_response(data)
        else:
            self._logger.warning(f"Unknown message type: {msg_type}")

    def _handle_init(self, data: Dict[str, Any]) -> None:
        self.strategy_name = data.get("strategy_name", "")
        self.strategy.name = self.strategy_name
        self.strategy.symbols = data.get("symbols", [])
        self.strategy.parameters = data.get("parameters", {})

        self._logger.info(f"Initializing strategy: {self.strategy_name}")

        ctx = Context(self)
        try:
            self.strategy.on_init(ctx)
            self._send_message("init_ack", {})
            self._logger.info("Strategy initialized successfully")
        except Exception as e:
            self._logger.error(f"Strategy initialization failed: {e}", exc_info=True)
            self._send_message("init_ack", {"error": str(e)})
            raise

    def _handle_history(self, data: Dict[str, Any]) -> None:
        history = HistoryData.from_dict(data)
        ctx = Context(self)
        try:
            self.strategy.on_history(ctx, history)
        except Exception as e:
            self._logger.error(f"Strategy on_history failed: {e}", exc_info=True)
            self._record_crash()

    def _handle_kline(self, data: Dict[str, Any]) -> None:
        kline = KlineData.from_dict(data)
        ctx = Context(self)
        try:
            signal = self.strategy.on_kline(ctx, kline)
            if signal is not None:
                self.send_signal(signal)
        except Exception as e:
            self._logger.error(f"Strategy on_kline failed: {e}", exc_info=True)
            self._record_crash()

    def _handle_tick(self, data: Dict[str, Any]) -> None:
        tick = TickData.from_dict(data)
        ctx = Context(self)
        try:
            signal = self.strategy.on_tick(ctx, tick)
            if signal is not None:
                self.send_signal(signal)
        except Exception as e:
            self._logger.error(f"Strategy tick failed: {e}", exc_info=True)
            self._record_crash()

    def _handle_stop(self, data: Dict[str, Any]) -> None:
        self._logger.info("Stopping strategy...")
        ctx = Context(self)
        try:
            self.strategy.on_stop(ctx)
            self._logger.info("Strategy stopped successfully")
        except Exception as e:
            self._logger.error(f"Strategy stop failed: {e}", exc_info=True)

    def _handle_rpc_response(self, data: Dict[str, Any]) -> None:
        request_id = data.get("request_id")
        if not request_id:
            self._logger.warning("RPC response missing request_id")
            return

        with self._rpc_lock:
            queue = self._rpc_pending.get(request_id)
            if not queue:
                self._logger.warning(f"No pending RPC for request_id: {request_id}")
                return
            queue.put(data)

    def rpc_call(self, method: str, params: Dict[str, Any], timeout: float = 5.0) -> Any:
        with self._rpc_lock:
            self._rpc_counter += 1
            request_id = f"rpc_{self._rpc_counter}"
            response_queue: Queue = Queue()
            self._rpc_pending[request_id] = response_queue

        try:
            self._send_message(
                "rpc_request",
                {
                    "request_id": request_id,
                    "method": method,
                    "params": params,
                },
            )

            try:
                response = response_queue.get(timeout=timeout)
            except Empty:
                raise TimeoutError(f"RPC call timeout: {method}")

            if "error" in response:
                error = response["error"]
                raise RuntimeError(f"RPC error: {error.get('message', 'unknown')}")

            return response.get("result")

        finally:
            with self._rpc_lock:
                self._rpc_pending.pop(request_id, None)

    def _send_message(self, msg_type: str, data: Dict[str, Any]) -> None:
        msg = {
            "version": "1.0",
            "type": msg_type,
            "timestamp": int(time.time() * 1000),
            "data": data,
        }
        line = json.dumps(msg)
        with self._stdout_lock:
            print(line, flush=True)

    def send_signal(self, signal: Signal) -> None:
        self._send_message("signal", signal.to_dict())

    def send_log(self, level: str, message: str) -> None:
        self._send_message("log", {"level": level, "message": message})

    def _start_heartbeat(self) -> None:
        self._heartbeat_thread = threading.Thread(
            target=self._heartbeat_loop, daemon=True
        )
        self._heartbeat_thread.start()

    def _stop_heartbeat(self) -> None:
        if self._heartbeat_thread and self._heartbeat_thread.is_alive():
            self._heartbeat_thread.join(timeout=1.0)

    def _heartbeat_loop(self) -> None:
        while self.running:
            try:
                time.sleep(self._heartbeat_interval)
                if self.running:
                    self._send_message("heartbeat", {})
            except Exception as e:
                self._logger.error(f"Heartbeat error: {e}")

    def _record_crash(self) -> None:
        now = time.time()
        self._crash_times.append(now)
        self._crash_times = [
            t for t in self._crash_times if now - t < self._crash_window
        ]
        if len(self._crash_times) >= self._crash_limit:
            self._logger.error(
                f"Too many crashes ({self._crash_limit} in {self._crash_window}s), shutting down"
            )
            self.running = False
            sys.exit(1)
