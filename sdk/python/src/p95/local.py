"""Local file-based storage for p95 SDK."""

import json
import sqlite3
import threading
import time
import uuid
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional


class LocalWriter:
    """
    Writes metrics directly to local SQLite + JSON files.

    Directory structure:
        {logdir}/{project}/{run_name}/
            run.db      # SQLite database with metrics
            meta.json   # Run metadata (name, status, git info, etc.)
            config.json # Hyperparameters
    """

    def __init__(
        self,
        logdir: str,
        project: str,
        run_name: str,
        tags: Optional[List[str]] = None,
        config: Optional[Dict[str, Any]] = None,
        git_info: Optional[Dict[str, Any]] = None,
        system_info: Optional[Dict[str, Any]] = None,
    ):
        """
        Initialize the local writer.

        Args:
            logdir: Base directory for all logs
            project: Project name (used as directory name)
            run_name: Run name (used as directory name)
            tags: Optional list of tags
            config: Optional initial configuration
            git_info: Optional git information
            system_info: Optional system information
        """
        # Sanitize names for filesystem
        self._project = self._sanitize_name(project)
        self._run_name = self._sanitize_name(run_name)
        self._run_id = str(uuid.uuid4())

        # Create run directory
        self._run_dir = Path(logdir).expanduser() / self._project / self._run_name
        self._run_dir.mkdir(parents=True, exist_ok=True)

        # Initialize SQLite database
        self._db_path = self._run_dir / "run.db"
        self._db: Optional[sqlite3.Connection] = None
        self._lock = threading.Lock()
        self._init_database()

        # Write initial metadata
        self._write_meta(
            {
                "id": self._run_id,
                "name": run_name,
                "project": project,
                "status": "running",
                "tags": tags or [],
                "git_info": git_info,
                "system_info": system_info,
                "started_at": datetime.now(timezone.utc).isoformat(),
                "ended_at": None,
                "error_message": None,
            }
        )

        # Write initial config
        if config:
            self._write_config(config)
        else:
            self._write_config({})

    @property
    def run_id(self) -> str:
        """Return the run ID."""
        return self._run_id

    @property
    def run_dir(self) -> Path:
        """Return the run directory path."""
        return self._run_dir

    def _init_database(self) -> None:
        """Initialize the SQLite database schema."""
        self._db = sqlite3.connect(str(self._db_path), check_same_thread=False)
        self._db.execute("PRAGMA journal_mode=WAL")  # Better concurrent access
        self._db.execute("""
            CREATE TABLE IF NOT EXISTS metrics (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                time REAL NOT NULL,
                name TEXT NOT NULL,
                step INTEGER NOT NULL,
                value REAL NOT NULL
            )
        """)
        self._db.execute("""
            CREATE INDEX IF NOT EXISTS idx_metrics_name_step
            ON metrics(name, step)
        """)
        self._db.execute("""
            CREATE INDEX IF NOT EXISTS idx_metrics_time
            ON metrics(time)
        """)
        self._db.commit()

    def log_metrics(
        self,
        metrics: Dict[str, float],
        step: int,
        timestamp: Optional[float] = None,
    ) -> None:
        """
        Log metrics to the SQLite database.

        Args:
            metrics: Dictionary of metric name -> value
            step: Step number
            timestamp: Unix timestamp (current time if not provided)
        """
        ts = timestamp or time.time()
        rows = [(ts, name, step, float(value)) for name, value in metrics.items()]

        with self._lock:
            if self._db is None:
                return
            self._db.executemany(
                "INSERT INTO metrics (time, name, step, value) VALUES (?, ?, ?, ?)",
                rows,
            )
            self._db.commit()

    def log_config(self, config: Dict[str, Any]) -> None:
        """
        Merge new config with existing config.

        Args:
            config: Configuration to merge
        """
        existing = self._read_config()
        existing.update(config)
        self._write_config(existing)

    def update_status(
        self,
        status: str,
        error: Optional[str] = None,
    ) -> None:
        """
        Update the run status.

        Args:
            status: New status (completed, failed, aborted)
            error: Optional error message
        """
        meta = self._read_meta()
        meta["status"] = status
        meta["ended_at"] = datetime.now(timezone.utc).isoformat()
        if error:
            meta["error_message"] = error

        # Calculate duration
        if meta.get("started_at"):
            started = datetime.fromisoformat(meta["started_at"])
            ended = datetime.now(timezone.utc)
            meta["duration_seconds"] = (ended - started).total_seconds()

        self._write_meta(meta)

    def close(self) -> None:
        """Close the database connection."""
        with self._lock:
            if self._db:
                self._db.close()
                self._db = None

    def _read_meta(self) -> Dict[str, Any]:
        """Read metadata from JSON file."""
        meta_path = self._run_dir / "meta.json"
        if meta_path.exists():
            return json.loads(meta_path.read_text())
        return {}

    def _write_meta(self, meta: Dict[str, Any]) -> None:
        """Write metadata to JSON file."""
        meta_path = self._run_dir / "meta.json"
        meta_path.write_text(json.dumps(meta, indent=2, default=str))

    def _read_config(self) -> Dict[str, Any]:
        """Read config from JSON file."""
        config_path = self._run_dir / "config.json"
        if config_path.exists():
            return json.loads(config_path.read_text())
        return {}

    def _write_config(self, config: Dict[str, Any]) -> None:
        """Write config to JSON file."""
        config_path = self._run_dir / "config.json"
        config_path.write_text(json.dumps(config, indent=2, default=str))

    def record_continuation(
        self,
        step: int,
        config_before: Dict[str, Any],
        config_after: Dict[str, Any],
        note: Optional[str] = None,
        git_info: Optional[Dict[str, Any]] = None,
        system_info: Optional[Dict[str, Any]] = None,
    ) -> Dict[str, Any]:
        """
        Record a continuation event.

        Args:
            step: The metric step at continuation
            config_before: Config snapshot before continuation
            config_after: Config snapshot after continuation
            note: Optional user note
            git_info: Optional git information at continuation time
            system_info: Optional system information at continuation time

        Returns:
            The created continuation record
        """
        continuations = self._read_continuations()
        continuation = {
            "id": str(uuid.uuid4()),
            "step": step,
            "timestamp": datetime.now(timezone.utc).isoformat(),
            "config_before": config_before,
            "config_after": config_after,
            "note": note,
            "git_info": git_info,
            "system_info": system_info,
        }
        continuations.append(continuation)
        self._write_continuations(continuations)
        return continuation

    def get_last_step(self) -> int:
        """
        Get the last step from the metrics database.

        Returns:
            The maximum step value, or 0 if no metrics exist
        """
        with self._lock:
            if self._db is None:
                return 0
            cursor = self._db.execute("SELECT MAX(step) FROM metrics")
            result = cursor.fetchone()
            return result[0] if result[0] is not None else 0

    def _read_continuations(self) -> List[Dict[str, Any]]:
        """Read continuations from JSON file."""
        continuations_path = self._run_dir / "continuations.json"
        if continuations_path.exists():
            return json.loads(continuations_path.read_text())
        return []

    def _write_continuations(self, continuations: List[Dict[str, Any]]) -> None:
        """Write continuations to JSON file."""
        continuations_path = self._run_dir / "continuations.json"
        continuations_path.write_text(json.dumps(continuations, indent=2, default=str))

    @classmethod
    def from_existing(
        cls,
        run_dir: Path,
        run_id: Optional[str] = None,
    ) -> "LocalWriter":
        """
        Create a LocalWriter instance for an existing run directory.

        This is used when resuming a run.

        Args:
            run_dir: Path to the existing run directory
            run_id: Optional run ID to use (reads from meta.json if not provided)

        Returns:
            A LocalWriter instance for the existing run
        """
        # Create a minimal instance without initializing a new run
        instance = object.__new__(cls)
        instance._run_dir = Path(run_dir)
        instance._lock = threading.Lock()

        # Read existing metadata
        meta = instance._read_meta()
        instance._run_id = run_id or meta.get("id", str(uuid.uuid4()))
        instance._project = meta.get("project", "unknown")
        instance._run_name = meta.get("name", "unknown")

        # Initialize database connection
        instance._db_path = instance._run_dir / "run.db"
        instance._db = None
        instance._init_database()

        return instance

    @staticmethod
    def _sanitize_name(name: str) -> str:
        """
        Sanitize a name for use as a directory name.

        Replaces unsafe characters with underscores.
        """
        # Replace path separators and other unsafe chars
        unsafe_chars = ["/", "\\", ":", "*", "?", '"', "<", ">", "|", "\0"]
        result = name
        for char in unsafe_chars:
            result = result.replace(char, "_")
        # Collapse multiple underscores
        while "__" in result:
            result = result.replace("__", "_")
        # Strip leading/trailing underscores and dots
        result = result.strip("_.")
        return result or "unnamed"


class LocalBatcher:
    """
    Batches metrics for efficient local writes.

    Similar to MetricsBatcher but writes to local SQLite instead of HTTP.
    """

    def __init__(
        self,
        writer: LocalWriter,
        batch_size: int = 100,
        flush_interval: float = 1.0,
    ):
        """
        Initialize the local batcher.

        Args:
            writer: The local writer instance
            batch_size: Number of metrics to batch before writing
            flush_interval: Seconds between automatic flushes
        """
        self._writer = writer
        self._batch_size = batch_size
        self._flush_interval = flush_interval

        self._buffer: List[tuple] = []  # (name, value, step, timestamp)
        self._lock = threading.Lock()
        self._stop_event = threading.Event()
        self._flush_thread: Optional[threading.Thread] = None

    def start(self) -> None:
        """Start the background flush thread."""
        self._flush_thread = threading.Thread(
            target=self._flush_loop,
            daemon=True,
            name="p95-local-flusher",
        )
        self._flush_thread.start()

    def stop(self) -> None:
        """Stop the background flush thread and flush remaining metrics."""
        self._stop_event.set()
        if self._flush_thread:
            self._flush_thread.join(timeout=10)
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
        ts = timestamp or time.time()

        with self._lock:
            self._buffer.append((name, value, step, ts))

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

        # Group by step for efficient writing
        metrics_by_step: Dict[int, Dict[str, tuple]] = {}
        for name, value, step, ts in self._buffer:
            if step not in metrics_by_step:
                metrics_by_step[step] = {}
            metrics_by_step[step][name] = (value, ts)

        self._buffer.clear()

        # Write each step's metrics
        for step, metrics in metrics_by_step.items():
            # Use the latest timestamp for this step
            latest_ts = max(ts for _, ts in metrics.values())
            metric_dict = {name: value for name, (value, _) in metrics.items()}
            self._writer.log_metrics(metric_dict, step, latest_ts)

    def _flush_loop(self) -> None:
        """Background thread that flushes periodically."""
        while not self._stop_event.wait(self._flush_interval):
            self.flush()

    @property
    def pending_count(self) -> int:
        """Return the number of pending metrics in the buffer."""
        with self._lock:
            return len(self._buffer)
