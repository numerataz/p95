"""Configuration management for the p95 SDK."""

import os
from dataclasses import dataclass, field
from pathlib import Path
from typing import Literal, Optional, Set


def _default_logdir() -> str:
    """Get the default log directory based on platform."""
    import platform

    home = Path.home()
    system = platform.system()

    if system == "Darwin":
        return str(home / "Library" / "Application Support" / "p95" / "logs")
    elif system == "Windows":
        appdata = os.environ.get("LOCALAPPDATA", str(home / "AppData" / "Local"))
        return str(Path(appdata) / "p95" / "logs")
    else:  # Linux and others
        xdg_data = os.environ.get("XDG_DATA_HOME", str(home / ".local" / "share"))
        return str(Path(xdg_data) / "p95" / "logs")


@dataclass
class SDKConfig:
    """SDK configuration settings."""

    # Mode: "local" or "remote"
    mode: Literal["local", "remote"] = "local"

    # Remote mode settings
    base_url: str = "http://localhost:8080"
    api_key: Optional[str] = None

    # Local mode settings
    logdir: str = field(default_factory=_default_logdir)

    # Common settings
    batch_size: int = 100
    flush_interval: float = 5.0
    capture_git: bool = True
    capture_system: bool = True

    # Remote-only settings
    timeout: int = 30
    retry_count: int = 3
    retry_delay: float = 1.0


# Global configuration instance and set of fields explicitly set via configure()
_config = SDKConfig()
_explicitly_set: Set[str] = set()


def configure(
    mode: Optional[Literal["local", "remote"]] = None,
    base_url: Optional[str] = None,
    api_key: Optional[str] = None,
    logdir: Optional[str] = None,
    batch_size: Optional[int] = None,
    flush_interval: Optional[float] = None,
    capture_git: Optional[bool] = None,
    capture_system: Optional[bool] = None,
    timeout: Optional[int] = None,
    retry_count: Optional[int] = None,
    retry_delay: Optional[float] = None,
) -> SDKConfig:
    """
    Configure the SDK globally.

    Explicit calls to configure() take priority over environment variables.

    Args:
        mode: Operating mode ("local" for file-based, "remote" for API server)
        base_url: Base URL for the p95 API server (remote mode)
        api_key: API key for authentication (remote mode)
        logdir: Directory for storing logs (local mode)
        batch_size: Number of metrics to batch before sending/writing
        flush_interval: Seconds between automatic flushes
        capture_git: Whether to capture git information
        capture_system: Whether to capture system information
        timeout: Request timeout in seconds (remote mode)
        retry_count: Number of retries for failed requests (remote mode)
        retry_delay: Delay between retries in seconds (remote mode)

    Returns:
        The updated configuration

    Example (local mode - default):
        from p95 import configure

        configure(logdir="./logs")  # Use local file storage

    Example (remote mode):
        from p95 import configure

        configure(
            mode="remote",
            base_url="https://api.p95.ai",
            api_key="p95_xxxx",
        )
    """
    global _config, _explicitly_set

    if mode is not None:
        _config.mode = mode
        _explicitly_set.add("mode")
    if base_url is not None:
        _config.base_url = base_url
        _explicitly_set.add("base_url")
    if api_key is not None:
        _config.api_key = api_key
        _explicitly_set.add("api_key")
    if logdir is not None:
        _config.logdir = logdir
        _explicitly_set.add("logdir")
    if batch_size is not None:
        _config.batch_size = batch_size
    if flush_interval is not None:
        _config.flush_interval = flush_interval
    if capture_git is not None:
        _config.capture_git = capture_git
    if capture_system is not None:
        _config.capture_system = capture_system
    if timeout is not None:
        _config.timeout = timeout
    if retry_count is not None:
        _config.retry_count = retry_count
    if retry_delay is not None:
        _config.retry_delay = retry_delay

    return _config


def _detect_mode() -> Literal["local", "remote"]:
    """
    Auto-detect the operating mode based on environment variables.

    Priority:
    1. P95_LOGDIR set -> local mode
    2. P95_URL or P95_API_KEY set -> remote mode
    3. Default -> local mode (zero config experience)
    """
    if os.environ.get("P95_LOGDIR"):
        return "local"
    if os.environ.get("P95_URL") or os.environ.get("P95_API_KEY"):
        return "remote"
    return "local"


def get_config() -> SDKConfig:
    """Get the current SDK configuration.

    configure() takes highest priority. Environment variables fill in any
    fields not explicitly set. Hardcoded defaults are used for the rest.
    """
    global _config

    if "mode" not in _explicitly_set:
        _config.mode = _detect_mode()
    if "logdir" not in _explicitly_set and os.environ.get("P95_LOGDIR"):
        _config.logdir = os.environ["P95_LOGDIR"]
    if "base_url" not in _explicitly_set and os.environ.get("P95_URL"):
        _config.base_url = os.environ["P95_URL"]
    if "api_key" not in _explicitly_set and os.environ.get("P95_API_KEY"):
        _config.api_key = os.environ["P95_API_KEY"]

    return _config
