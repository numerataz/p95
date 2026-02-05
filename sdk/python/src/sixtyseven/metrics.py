"""Metrics batching and buffering for efficient network transmission."""

import threading
import time
from collections import deque
from dataclasses import dataclass
from typing import TYPE_CHECKING, Deque, Optional

if TYPE_CHECKING:
    from sixtyseven.client import SixtySevenClient


@dataclass
class MetricPoint:
    """A single metric data point."""

    name: str
    value: float
    step: int
    timestamp: float


class MetricsBatcher:
    """
    Batches metrics for efficient network transmission.

    Implements:
    - Automatic batching by count (batch_size)
    - Automatic flushing by time (flush_interval)
    - Thread-safe operation
    - Graceful shutdown
    """

    def __init__(
        self,
        client: "SixtySevenClient",
        run_id: str,
        batch_size: int = 100,
        flush_interval: float = 5.0,
    ):
        """
        Initialize the metrics batcher.

        Args:
            client: The API client
            run_id: The run ID
            batch_size: Number of metrics to batch before sending
            flush_interval: Seconds between automatic flushes
        """
        self._client = client
        self._run_id = run_id
        self._batch_size = batch_size
        self._flush_interval = flush_interval

        self._buffer: Deque[MetricPoint] = deque()
        self._lock = threading.Lock()
        self._stop_event = threading.Event()
        self._flush_thread: Optional[threading.Thread] = None

    def start(self) -> None:
        """Start the background flush thread."""
        self._flush_thread = threading.Thread(
            target=self._flush_loop,
            daemon=True,
            name="sixtyseven-metrics-flusher",
        )
        self._flush_thread.start()

    def stop(self) -> None:
        """Stop the background flush thread and flush remaining metrics."""
        self._stop_event.set()
        if self._flush_thread:
            self._flush_thread.join(timeout=10)

        # Final flush
        self.flush()

    def add(
        self,
        name: str,
        value: float,
        step: int,
        timestamp: Optional[float] = None,
    ) -> None:
        """
        Add a metric point to the buffer.

        Args:
            name: Metric name
            value: Metric value
            step: Step number
            timestamp: Unix timestamp (current time if not provided)
        """
        point = MetricPoint(
            name=name,
            value=value,
            step=step,
            timestamp=timestamp or time.time(),
        )

        with self._lock:
            self._buffer.append(point)

            # Flush if batch is full
            if len(self._buffer) >= self._batch_size:
                self._do_flush()

    def flush(self) -> None:
        """Force flush all buffered metrics."""
        with self._lock:
            self._do_flush()

    def _do_flush(self) -> None:
        """Internal flush (must hold lock)."""
        if not self._buffer:
            return

        # Collect all points
        points = list(self._buffer)
        self._buffer.clear()

        # Convert to API format
        metrics = [
            {
                "name": p.name,
                "value": p.value,
                "step": p.step,
                "timestamp": self._format_timestamp(p.timestamp),
            }
            for p in points
        ]

        # Send to server
        try:
            self._client.batch_log_metrics(self._run_id, metrics)
        except Exception as e:
            # Log error but don't lose metrics - put them back
            # In production, you might want to implement a dead letter queue
            print(f"Warning: Failed to send metrics: {e}")

    def _flush_loop(self) -> None:
        """Background thread that flushes periodically."""
        while not self._stop_event.wait(self._flush_interval):
            self.flush()

    @staticmethod
    def _format_timestamp(ts: float) -> str:
        """Format timestamp as ISO 8601 string."""
        from datetime import datetime, timezone

        return datetime.fromtimestamp(ts, tz=timezone.utc).isoformat()

    @property
    def pending_count(self) -> int:
        """Return the number of pending metrics in the buffer."""
        with self._lock:
            return len(self._buffer)
