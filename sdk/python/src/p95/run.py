"""Main Run class for tracking ML experiments."""

from __future__ import annotations

import atexit
import os
import signal
import sys
import threading
import time
from typing import Any, Dict, List, Optional, TYPE_CHECKING

from p95.config import SDKConfig, get_config
from p95.exceptions import EarlyStopException, ValidationError
from p95.utils import generate_run_name, get_git_info, get_system_info

if TYPE_CHECKING:
    from p95.client import P95Client
    from p95.metrics import MetricsBatcher
    from p95.local import LocalWriter, LocalBatcher
    from p95.server import ServerManager


class Run:
    """
    Main class for tracking ML experiments with p95.

    A Run represents a single training session or experiment. It tracks:
    - Configuration/hyperparameters
    - Metrics over time (loss, accuracy, etc.)
    - System information
    - Git information

    Supports two modes:
    - Local mode (default): Writes to local SQLite files, viewable with `pnf --logdir`
    - Remote mode: Sends to a p95 API server

    Usage:
        # Context manager (recommended)
        with Run(project="my-project") as run:
            run.log_config({"lr": 0.001})
            for epoch in range(100):
                run.log_metrics({"loss": 0.5}, step=epoch)

        # Explicit management
        run = Run(project="my-project")
        run.log_metrics({"loss": 0.5}, step=1)
        run.complete()  # or run.fail("error message")

    Environment variables:
        P95_LOGDIR: Set to enable local mode with custom log directory
        P95_URL: Set to enable remote mode with API server
        P95_API_KEY: API key for remote mode authentication
    """

    def __init__(
        self,
        project: str,
        api_key: Optional[str] = None,
        name: Optional[str] = None,
        tags: Optional[List[str]] = None,
        config: Optional[Dict[str, Any]] = None,
        # Advanced options
        mode: Optional[str] = None,  # "local" or "remote"
        logdir: Optional[str] = None,
        base_url: Optional[str] = None,
        batch_size: Optional[int] = None,
        flush_interval: Optional[float] = None,
        capture_git: Optional[bool] = None,
        capture_system: Optional[bool] = None,
        # Server option (local mode only)
        start_server: bool = False,
        start_tui: bool = False,
        # Sharing option (remote mode only)
        share: bool = False,
    ):
        """
        Initialize a new run.

        Args:
            project: Project identifier. In local mode, this is just a project name.
                     In remote mode, use "team-slug/app-slug" format.
            api_key: API key for authentication (remote mode only)
            name: Run name (auto-generated if not provided)
            tags: List of tags for organizing runs
            config: Initial configuration/hyperparameters
            mode: Operating mode ("local" or "remote"). Auto-detected if not specified.
            logdir: Directory for logs (local mode only)
            base_url: API base URL (remote mode only)
            batch_size: Number of metrics to batch before sending/writing
            flush_interval: Seconds between automatic flushes
            capture_git: Whether to capture git information
            capture_system: Whether to capture system information
            start_server: Automatically start the p95 viewer server and open the
                browser (local mode only). The server stops when the run ends.
            start_tui: Automatically open the p95 TUI in a new terminal window
                (local mode only). The TUI manages its own internal server.
            share: Automatically create a public share link when the run finishes
                (remote mode only). The link is printed to stdout in the form
                https://p95.run/{slug}.

        Raises:
            ValidationError: If project format is invalid (remote mode)
            AuthenticationError: If API key is invalid (remote mode)
            ServerError: If start_server=True but the pnf binary cannot be found
        """
        # Get global config
        global_config = get_config()

        # Build effective config
        self._config = SDKConfig(
            mode=mode or global_config.mode,
            logdir=logdir or global_config.logdir,
            base_url=base_url or global_config.base_url,
            api_key=api_key or global_config.api_key,
            batch_size=batch_size or global_config.batch_size,
            flush_interval=flush_interval or global_config.flush_interval,
            capture_git=capture_git
            if capture_git is not None
            else global_config.capture_git,
            capture_system=capture_system
            if capture_system is not None
            else global_config.capture_system,
        )

        self._project = project
        self._run_name = name or generate_run_name()
        self._tags = tags or []
        self._initial_config = config

        # Check for sweep context - auto-link to sweep if running inside agent
        self._sweep_id: Optional[str] = None
        self._sweep_params: Optional[Dict[str, Any]] = None
        self._sweep_context = None
        self._check_sweep_context()

        # Mode-specific initialization
        self._local_writer: Optional["LocalWriter"] = None
        self._local_batcher: Optional["LocalBatcher"] = None
        self._remote_client: Optional["P95Client"] = None
        self._remote_batcher: Optional["MetricsBatcher"] = None
        self._server_manager: Optional["ServerManager"] = None
        self._start_server = start_server
        self._start_tui = start_tui
        self._share = share

        # Capture info before creating run
        self._git_info = None
        self._system_info = None
        if self._config.capture_git:
            self._git_info = get_git_info()
        if self._config.capture_system:
            self._system_info = get_system_info()

        if self._config.mode == "local":
            self._init_local_mode()
        else:
            self._init_remote_mode()

        # Track state
        self._step = 0
        self._closed = False
        self._lock = threading.Lock()

        # Register cleanup handlers
        atexit.register(self._cleanup)
        self._setup_signal_handlers()

    def _init_local_mode(self) -> None:
        """Initialize local file-based storage."""
        from p95.local import LocalWriter, LocalBatcher

        self._local_writer = LocalWriter(
            logdir=self._config.logdir,
            project=self._project,
            run_name=self._run_name,
            tags=self._tags,
            config=self._initial_config,
            git_info=self._git_info,
            system_info=self._system_info,
        )
        self._run_id = self._local_writer.run_id

        self._local_batcher = LocalBatcher(
            writer=self._local_writer,
            batch_size=self._config.batch_size,
            flush_interval=self._config.flush_interval,
        )
        self._local_batcher.start()

        # Print local mode info
        print(f"p95: Logging to {self._local_writer.run_dir}")

        # start_tui launches after training completes (see _finalize)
        if self._start_tui:
            from p95.server import ServerManager

            self._server_manager = ServerManager(
                logdir=self._config.logdir,
                open_tui=True,
                project=self._project,
                run_id=self._run_id,
            )
        elif self._start_server:
            from p95.server import ServerManager

            self._server_manager = ServerManager(
                logdir=self._config.logdir,
                open_browser=True,
                project=self._project,
                run_id=self._run_id,
                keep_running=True,  # Don't stop server when run ends (TensorBoard-style)
            )
            self._server_manager.start()

    def _init_remote_mode(self) -> None:
        """Initialize remote API client."""
        from p95.client import P95Client
        from p95.metrics import MetricsBatcher

        # Parse project for remote mode
        self._team_slug, self._app_slug = self._parse_project(self._project)

        self._remote_client = P95Client(self._config, self._config.api_key)

        # Create run on server
        self._run_id = self._remote_client.create_run(
            team_slug=self._team_slug,
            app_slug=self._app_slug,
            name=self._run_name,
            tags=self._tags,
            config=self._initial_config or {},
            git_info=self._git_info,
            system_info=self._system_info,
        )

        # Auto-link to job if running within a job context
        job_id = os.environ.get("P95_JOB_ID")
        if job_id:
            try:
                self._remote_client.link_run_to_job(job_id, self._run_id)
            except Exception as e:
                # Log but don't fail - the run was created successfully
                print(f"p95: Warning: Failed to link run to job {job_id}: {e}")

        self._remote_batcher = MetricsBatcher(
            client=self._remote_client,
            run_id=self._run_id,
            batch_size=self._config.batch_size,
            flush_interval=self._config.flush_interval,
        )
        self._remote_batcher.start()

    def _check_sweep_context(self) -> None:
        """Check if running inside a sweep agent and auto-link."""
        try:
            from p95.sweep import get_current_sweep_context
        except ImportError:
            return

        ctx = get_current_sweep_context()
        if ctx is None:
            return

        # Auto-link to sweep
        self._sweep_id = ctx.sweep_id
        self._sweep_params = ctx.params
        self._sweep_context = ctx

        # Merge sweep params into config
        if ctx.params:
            if self._initial_config is None:
                self._initial_config = {}
            # Sweep params take precedence
            merged = {**self._initial_config, **ctx.params}
            # Also merge any static sweep config
            if ctx.sweep_data and ctx.sweep_data.get("config"):
                merged = {**ctx.sweep_data["config"], **merged}
            self._initial_config = merged

        # Register this run with the context so agent can track it
        ctx._run = self

        print(f"p95: Run auto-linked to sweep {ctx.sweep_id[:8]}...")

    @property
    def id(self) -> str:
        """Return the run ID."""
        return self._run_id

    @property
    def name(self) -> str:
        """Return the run name."""
        return self._run_name

    @property
    def project(self) -> str:
        """Return the project identifier."""
        return self._project

    @property
    def mode(self) -> str:
        """Return the operating mode ('local' or 'remote')."""
        return self._config.mode

    @property
    def logdir(self) -> Optional[str]:
        """Return the log directory (local mode only)."""
        if self._local_writer:
            return str(self._local_writer.run_dir)
        return None

    def log_config(self, config: Dict[str, Any]) -> None:
        """
        Log configuration/hyperparameters.

        This merges with any config provided at initialization.
        Can be called multiple times to add more config.

        Args:
            config: Dictionary of configuration values

        Example:
            run.log_config({
                "learning_rate": 0.001,
                "batch_size": 32,
                "optimizer": "adam",
            })
        """
        if self._config.mode == "local":
            self._local_writer.log_config(config)
        else:
            self._remote_client.update_run_config(self._run_id, config)

    def log_metrics(
        self,
        metrics: Dict[str, float],
        step: Optional[int] = None,
        timestamp: Optional[float] = None,
    ) -> None:
        """
        Log metrics for the current step.

        Metrics are automatically batched for efficiency. Use flush()
        to force immediate sending.

        Args:
            metrics: Dictionary of metric name -> value
            step: Step number (auto-incremented if not provided)
            timestamp: Unix timestamp (current time if not provided)

        Example:
            run.log_metrics({
                "train/loss": 0.45,
                "train/accuracy": 0.82,
                "val/loss": 0.52,
            }, step=epoch)
        """
        with self._lock:
            if step is None:
                step = self._step
                self._step += 1
            else:
                self._step = max(self._step, step + 1)

            ts = timestamp or time.time()

            batcher = (
                self._local_batcher
                if self._config.mode == "local"
                else self._remote_batcher
            )
            for name, value in metrics.items():
                batcher.add(name=name, value=float(value), step=step, timestamp=ts)

    def log(self, name: str, value: float, step: Optional[int] = None) -> None:
        """
        Log a single metric.

        Convenience method for logging one metric at a time.

        Args:
            name: Metric name
            value: Metric value
            step: Step number (auto-incremented if not provided)
        """
        self.log_metrics({name: value}, step=step)

    def flush(self) -> None:
        """Force flush all buffered metrics."""
        if self._config.mode == "local":
            self._local_batcher.flush()
        else:
            self._remote_batcher.flush()

    def add_tags(self, tags: List[str]) -> None:
        """
        Add tags to the run.

        Args:
            tags: List of tags to add
        """
        if self._config.mode == "local":
            # For local mode, update the meta.json
            meta = self._local_writer._read_meta()
            existing_tags = meta.get("tags", [])
            meta["tags"] = list(set(existing_tags + tags))
            self._local_writer._write_meta(meta)
        else:
            self._remote_client.add_run_tags(self._run_id, tags)

    def check_intervention(self) -> Optional[Dict[str, Any]]:
        """
        Check for a pending intervention from an AI agent.

        Call this periodically in your training loop to check if an AI
        or human has requested a config change, early stop, or pause.

        Returns:
            Intervention dictionary if pending, None otherwise.
            The dictionary contains:
            - id: Intervention ID
            - type: "adjust_config", "early_stop", "pause", "resume"
            - config_delta: Config changes to apply (for adjust_config)
            - rationale: Explanation for the intervention

        Example:
            for epoch in range(100):
                train_step()
                intervention = run.check_intervention()
                if intervention:
                    run.apply_intervention(intervention)
        """
        if self._config.mode == "local":
            # Local mode doesn't support interventions
            return None

        return self._remote_client.get_pending_intervention(self._run_id)

    def apply_intervention(self, intervention: Optional[Dict[str, Any]]) -> None:
        """
        Apply and acknowledge an intervention.

        This method:
        1. Updates the run's config if type is "adjust_config"
        2. Raises EarlyStopException if type is "early_stop"
        3. Marks the intervention as applied on the server

        Args:
            intervention: Intervention dictionary from check_intervention()

        Raises:
            EarlyStopException: If the intervention is an early stop request

        Example:
            try:
                for epoch in range(100):
                    train_step()
                    intervention = run.check_intervention()
                    if intervention:
                        run.apply_intervention(intervention)
            except EarlyStopException as e:
                print(f"Training stopped early: {e.rationale}")
        """
        if intervention is None:
            return

        if self._config.mode == "local":
            # Local mode doesn't support interventions
            return

        intervention_type = intervention.get("type")
        rationale = intervention.get("rationale", "")

        if intervention_type == "adjust_config":
            config_delta = intervention.get("config_delta", {})
            if config_delta:
                # Update config on the server
                self._remote_client.update_run_config(self._run_id, config_delta)
                # Update local config tracking if exists
                if self._initial_config is not None:
                    self._initial_config.update(config_delta)

        # Acknowledge the intervention
        self._remote_client.ack_intervention(intervention["id"])

        if intervention_type == "early_stop":
            raise EarlyStopException(rationale)

    @property
    def config(self) -> Optional[Dict[str, Any]]:
        """Return the current run configuration."""
        return self._initial_config

    def complete(self) -> None:
        """Mark the run as completed successfully."""
        self._finalize("completed")

    def fail(self, error: Optional[str] = None) -> None:
        """
        Mark the run as failed.

        Args:
            error: Optional error message
        """
        self._finalize("failed", error=error)

    def abort(self) -> None:
        """Mark the run as aborted."""
        self._finalize("aborted")

    def _finalize(self, status: str, error: Optional[str] = None) -> None:
        """Finalize the run with the given status."""
        with self._lock:
            if self._closed:
                return
            self._closed = True

        if self._config.mode == "local":
            # Flush and stop local batcher
            self._local_batcher.flush()
            self._local_batcher.stop()
            # Update status in meta.json
            self._local_writer.update_status(status, error)
            self._local_writer.close()
            # Note: We don't stop the server here - it keeps running (TensorBoard-style)
            # so users can view results after training ends
            # Launch TUI after the script fully exits
            if self._server_manager is not None and self._start_tui:
                atexit.register(self._server_manager.start)
            if self._share:
                print("p95: Warning: share=True is only available in remote mode.")
        else:
            # Flush and stop remote batcher
            self._remote_batcher.flush()
            self._remote_batcher.stop()
            # Update status on server
            self._remote_client.update_run_status(self._run_id, status, error=error)
            if self._share:
                try:
                    share_response = self._remote_client.share_run(self._run_id)
                    slug = share_response.get("slug")
                    if slug:
                        print(f"p95: Share your run at https://p95.run/{slug}")
                except Exception as e:
                    print(f"p95: Warning: Failed to create share link: {e}")

    def __enter__(self) -> "Run":
        """Enter context manager."""
        return self

    def __exit__(self, exc_type, exc_val, exc_tb) -> None:
        """Exit context manager, marking run as completed or failed."""
        if exc_type is not None:
            self.fail(str(exc_val) if exc_val else "Unknown error")
        else:
            self.complete()

    def _setup_signal_handlers(self) -> None:
        """Set up signal handlers for graceful shutdown."""
        self._original_sigint = signal.getsignal(signal.SIGINT)
        self._original_sigterm = signal.getsignal(signal.SIGTERM)

        def handle_signal(signum, frame):
            """Handle termination signals."""
            # Immediately restore original handlers to prevent re-entry
            signal.signal(signal.SIGINT, self._original_sigint)
            signal.signal(signal.SIGTERM, self._original_sigterm)

            # Mark run as canceled
            if not self._closed:
                sig_name = "SIGINT" if signum == signal.SIGINT else "SIGTERM"
                print(f"\np95: Run canceled ({sig_name})")
                self._finalize("canceled", error=f"Interrupted by {sig_name}")

            # Re-raise to exit
            if signum == signal.SIGINT:
                raise KeyboardInterrupt
            else:
                sys.exit(128 + signum)

        # Only set handlers in main thread
        if threading.current_thread() is threading.main_thread():
            signal.signal(signal.SIGINT, handle_signal)
            signal.signal(signal.SIGTERM, handle_signal)

    def _cleanup(self) -> None:
        """Cleanup handler for atexit - marks incomplete runs as canceled."""
        if not self._closed:
            self._finalize("canceled", error="Process exited unexpectedly")

    @staticmethod
    def _parse_project(project: str) -> tuple:
        """Parse project string into team and app slugs (remote mode only)."""
        parts = project.split("/")
        if len(parts) != 2:
            raise ValidationError(
                f"Invalid project format: '{project}'. Expected 'team-slug/app-slug' format for remote mode."
            )
        return parts[0], parts[1]

    def __repr__(self) -> str:
        mode_info = (
            f"logdir='{self.logdir}'"
            if self._config.mode == "local"
            else f"url='{self._config.base_url}'"
        )
        return f"Run(id='{self._run_id}', project='{self.project}', mode='{self.mode}', {mode_info})"

    @classmethod
    def _resume_local(
        cls,
        run_path: str,
        config: Optional[Dict[str, Any]] = None,
        note: Optional[str] = None,
        **kwargs,
    ) -> "Run":
        """
        Resume an existing local run.

        Args:
            run_path: Path to the run directory
            config: New/updated config (merged with existing)
            note: Optional continuation note
            **kwargs: Additional options

        Returns:
            Active Run object
        """
        from pathlib import Path
        from p95.local import LocalWriter, LocalBatcher

        run_dir = Path(run_path).expanduser()
        if not run_dir.exists():
            raise ValidationError(f"Run directory not found: {run_path}")

        meta_path = run_dir / "meta.json"
        if not meta_path.exists():
            raise ValidationError(f"Run metadata not found: {meta_path}")

        import json

        meta = json.loads(meta_path.read_text())

        # Verify run is not currently running
        if meta.get("status") == "running":
            raise ValidationError("Run is already running")

        # Get global config
        global_config = get_config()

        # Build effective config
        sdk_config = SDKConfig(
            mode="local",
            logdir=str(run_dir.parent.parent),  # Go up to logdir
            base_url=kwargs.get("base_url") or global_config.base_url,
            api_key=kwargs.get("api_key") or global_config.api_key,
            batch_size=kwargs.get("batch_size") or global_config.batch_size,
            flush_interval=kwargs.get("flush_interval") or global_config.flush_interval,
            capture_git=kwargs.get("capture_git", global_config.capture_git),
            capture_system=kwargs.get("capture_system", global_config.capture_system),
        )

        # Create instance
        instance = object.__new__(cls)
        instance._config = sdk_config
        instance._project = meta.get("project", "unknown")
        instance._run_name = meta.get("name", "unknown")
        instance._tags = meta.get("tags", [])
        instance._initial_config = config

        # Remote mode objects (unused in local mode)
        instance._remote_client = None
        instance._remote_batcher = None
        instance._server_manager = None
        instance._start_server = kwargs.get("start_server", False)

        # Capture current git/system info
        instance._git_info = None
        instance._system_info = None
        if sdk_config.capture_git:
            instance._git_info = get_git_info()
        if sdk_config.capture_system:
            instance._system_info = get_system_info()

        # Create writer for existing run
        instance._local_writer = LocalWriter.from_existing(run_dir)
        instance._run_id = instance._local_writer.run_id

        # Get current step and config
        current_step = instance._local_writer.get_last_step()
        config_before = instance._local_writer._read_config()

        # Merge new config with existing
        config_after = dict(config_before)
        if config:
            config_after.update(config)
            instance._local_writer._write_config(config_after)

        # Record continuation
        instance._local_writer.record_continuation(
            step=current_step,
            config_before=config_before,
            config_after=config_after,
            note=note,
            git_info=instance._git_info,
            system_info=instance._system_info,
        )

        # Update status back to running
        meta["status"] = "running"
        meta["ended_at"] = None
        meta["error_message"] = None
        instance._local_writer._write_meta(meta)

        # Start batcher
        instance._local_batcher = LocalBatcher(
            writer=instance._local_writer,
            batch_size=sdk_config.batch_size,
            flush_interval=sdk_config.flush_interval,
        )
        instance._local_batcher.start()

        # Track state
        instance._step = current_step + 1
        instance._closed = False
        instance._lock = threading.Lock()

        # Register cleanup handlers
        atexit.register(instance._cleanup)
        instance._setup_signal_handlers()

        print(f"p95: Resumed run at {run_dir}")

        # Start server if requested
        if instance._start_server:
            from p95.server import ServerManager

            instance._server_manager = ServerManager(
                logdir=sdk_config.logdir,
                open_browser=True,
                project=instance._project,
                run_id=instance._run_id,
            )
            instance._server_manager.start()

        return instance

    @classmethod
    def _resume_remote(
        cls,
        run_id: str,
        config: Optional[Dict[str, Any]] = None,
        note: Optional[str] = None,
        **kwargs,
    ) -> "Run":
        """
        Resume an existing remote run.

        Args:
            run_id: The run ID to resume
            config: New/updated config (merged with existing)
            note: Optional continuation note
            **kwargs: Additional options

        Returns:
            Active Run object
        """
        from p95.client import P95Client
        from p95.metrics import MetricsBatcher

        # Get global config
        global_config = get_config()

        # Build effective config
        sdk_config = SDKConfig(
            mode="remote",
            logdir=kwargs.get("logdir") or global_config.logdir,
            base_url=kwargs.get("base_url") or global_config.base_url,
            api_key=kwargs.get("api_key") or global_config.api_key,
            batch_size=kwargs.get("batch_size") or global_config.batch_size,
            flush_interval=kwargs.get("flush_interval") or global_config.flush_interval,
            capture_git=kwargs.get("capture_git", global_config.capture_git),
            capture_system=kwargs.get("capture_system", global_config.capture_system),
        )

        # Create client
        client = P95Client(sdk_config, sdk_config.api_key)

        # Capture current git/system info
        git_info = None
        system_info = None
        if sdk_config.capture_git:
            git_info = get_git_info()
        if sdk_config.capture_system:
            system_info = get_system_info()

        # Call resume endpoint
        response = client.resume_run(
            run_id=run_id,
            config=config,
            note=note,
            git_info=git_info,
            system_info=system_info,
        )

        run_data = response.get("run", {})

        # Create instance
        instance = object.__new__(cls)
        instance._config = sdk_config
        instance._project = kwargs.get(
            "project", f"{run_data.get('app_id', 'unknown')}"
        )
        instance._run_name = run_data.get("name", "unknown")
        instance._tags = run_data.get("tags", [])
        instance._initial_config = config
        instance._run_id = run_id

        # Remote mode objects
        instance._remote_client = client
        instance._local_writer = None
        instance._local_batcher = None
        instance._server_manager = None
        instance._start_server = False
        instance._git_info = git_info
        instance._system_info = system_info

        # Start batcher
        instance._remote_batcher = MetricsBatcher(
            client=client,
            run_id=run_id,
            batch_size=sdk_config.batch_size,
            flush_interval=sdk_config.flush_interval,
        )
        instance._remote_batcher.start()

        # Track state - get step from continuation response
        continuation = response.get("continuation", {})
        instance._step = continuation.get("step", 0) + 1
        instance._closed = False
        instance._lock = threading.Lock()

        # Register cleanup handlers
        atexit.register(instance._cleanup)
        instance._setup_signal_handlers()

        return instance


def resume(
    run_id: str,
    config: Optional[Dict[str, Any]] = None,
    note: Optional[str] = None,
    **kwargs,
) -> Run:
    """
    Resume a previously completed/failed run.

    This allows continuing to log metrics to an existing run while recording
    a continuation event that captures config changes. The continuation point
    is visible on charts as a marker.

    Config is merged with existing config (overlay behavior).
    The continuation event captures config_before and config_after
    so the diff is always visible.

    Args:
        run_id: Run ID or path to run directory (local mode auto-detected by path)
        config: New/updated config (merged with existing)
        note: Optional continuation note
        **kwargs: Same options as Run() - api_key, base_url, batch_size, etc.

    Returns:
        Active Run object that can be used with context manager or explicit complete()/fail()

    Example:
        # Resume with new learning rate
        resumed = p95.resume(
            run.id,
            config={"lr": 0.0001},
            note="Reduced LR for fine-tuning"
        )

        with resumed:
            for epoch in range(100, 200):
                resumed.log_metrics({"loss": compute_loss()}, step=epoch)

    Raises:
        ValidationError: If run not found or already running
        AuthenticationError: If API key is invalid (remote mode)
    """
    from pathlib import Path

    # Determine if this is a local path or remote run ID
    # Local paths contain slashes or are absolute paths
    run_path = Path(run_id).expanduser()
    is_local_path = (
        run_path.exists() or "/" in run_id or "\\" in run_id or run_id.startswith("~")
    )

    # Also check if mode is explicitly set
    mode = kwargs.get("mode")
    if mode == "local" or (mode is None and is_local_path):
        return Run._resume_local(run_id, config=config, note=note, **kwargs)
    else:
        return Run._resume_remote(run_id, config=config, note=note, **kwargs)
