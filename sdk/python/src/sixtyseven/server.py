"""Server management for automatically starting the Sixtyseven viewer."""

from __future__ import annotations

import atexit
import os
import platform
import shutil
import socket
import subprocess
import time
from pathlib import Path
from typing import Optional

from sixtyseven.exceptions import ServerError


class ServerManager:
    """
    Manages the Sixtyseven server lifecycle.

    Handles discovering, starting, and stopping the sixtyseven binary
    for viewing training metrics in real-time.

    Usage:
        manager = ServerManager(logdir="./logs")
        manager.start()
        # ... do training ...
        manager.stop()  # or let it auto-cleanup on exit
    """

    BINARY_NAME = "sixtyseven"
    DEFAULT_PORT = 6767
    DEFAULT_HOST = "localhost"
    HEALTH_CHECK_TIMEOUT = 10  # seconds
    HEALTH_CHECK_INTERVAL = 0.2  # seconds

    def __init__(
        self,
        logdir: str,
        port: int = DEFAULT_PORT,
        host: str = DEFAULT_HOST,
        open_browser: bool = True,
        binary_path: Optional[str] = None,
        keep_running: bool = False,
        project: Optional[str] = None,
        run_id: Optional[str] = None,
    ):
        """
        Initialize the server manager.

        Args:
            logdir: Directory containing the logs to serve
            port: Port to run the server on
            host: Host to bind the server to
            open_browser: Whether to open the browser automatically
            binary_path: Explicit path to the sixtyseven binary (auto-discovered if not provided)
            keep_running: If True, don't stop the server when the manager is garbage collected
            project: Project name (used as fallback if run_id not provided)
            run_id: Run ID to open in the browser (opens specific run view)
        """
        self.logdir = logdir
        self.port = port
        self.host = host
        self.open_browser = open_browser
        self.binary_path = binary_path
        self.keep_running = keep_running
        self.project = project
        self.run_id = run_id

        self._process: Optional[subprocess.Popen] = None
        self._started = False
        self._we_started_server = False  # Track if we started it vs reusing existing

    def start(self) -> str:
        """
        Start the server.

        Returns:
            The URL where the server is running

        Raises:
            ServerError: If the binary cannot be found or the server fails to start
        """
        if self._started:
            return self.url

        # Check if a server is already running on this port
        if self._is_port_in_use():
            # Server already running - just set the active run so UI navigates
            self._set_active_run()
            if self.open_browser:
                self._open_browser()
            print(f"Sixtyseven: Navigating to run in existing viewer at {self.url}")
            self._started = True
            return self.url

        # Find the binary
        binary = self._find_binary()
        if not binary:
            raise ServerError(
                "Could not find 'sixtyseven' binary. Please ensure it's installed and in your PATH, "
                "or specify the path explicitly with server_binary='/path/to/sixtyseven'"
            )

        # Build the command (we handle browser opening ourselves for project-specific URLs)
        cmd = [
            binary,
            "serve",
            f"--logdir={self.logdir}",
            f"--port={self.port}",
            f"--host={self.host}",
            "--open=false",  # We'll open the browser ourselves with the right URL
        ]

        # Start the server process
        try:
            # Use DEVNULL for stdin to prevent the process from waiting for input
            # Redirect stdout/stderr to suppress server logs in the training output
            self._process = subprocess.Popen(
                cmd,
                stdin=subprocess.DEVNULL,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                start_new_session=True,  # Detach from parent process group
            )
        except OSError as e:
            raise ServerError(f"Failed to start server: {e}")

        # Wait for the server to be ready
        if not self._wait_for_health():
            # Server didn't start properly - get error output
            if self._process.poll() is not None:
                _, stderr = self._process.communicate()
                error_msg = stderr.decode().strip() if stderr else "Unknown error"
                raise ServerError(f"Server failed to start: {error_msg}")
            else:
                self._process.terminate()
                raise ServerError(
                    f"Server did not respond within {self.HEALTH_CHECK_TIMEOUT}s"
                )

        self._started = True
        self._we_started_server = True

        # Register cleanup handler (unless keep_running is True)
        if not self.keep_running:
            atexit.register(self.stop)

        # Set active run so UI navigates to it
        self._set_active_run()

        # Open browser to the project-specific URL
        if self.open_browser:
            self._open_browser()

        print(f"Sixtyseven: Server started at {self.url}")
        return self.url

    def stop(self) -> None:
        """Stop the server if it was started by this manager."""
        if not self._we_started_server:
            return  # Don't stop a server we didn't start

        if self._process is not None and self._process.poll() is None:
            self._process.terminate()
            try:
                self._process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self._process.kill()
            self._process = None
            self._we_started_server = False
            print("Sixtyseven: Server stopped")

    @property
    def url(self) -> str:
        """Return the server URL."""
        return f"http://{self.host}:{self.port}"

    @property
    def run_url(self) -> str:
        """Return the URL to open in the browser."""
        if self.run_id:
            return f"{self.url}/runs/{self.run_id}"
        if self.project:
            return f"{self.url}/projects/{self.project}"
        return f"{self.url}/projects"

    def _open_browser(self) -> None:
        """Open the browser to the run URL."""
        import webbrowser

        webbrowser.open(self.run_url)

    def _set_active_run(self) -> None:
        """Tell the server about the active run so the UI can navigate to it."""
        if not self.run_id:
            return

        import urllib.request
        import urllib.error
        import json

        url = f"{self.url}/api/v1/active-run"
        data = json.dumps({"run_id": self.run_id}).encode("utf-8")

        try:
            req = urllib.request.Request(
                url,
                data=data,
                headers={"Content-Type": "application/json"},
                method="POST",
            )
            with urllib.request.urlopen(req, timeout=2):
                pass
        except (urllib.error.URLError, OSError):
            # Server might not support this endpoint yet, ignore
            pass

    @property
    def is_running(self) -> bool:
        """Check if the server is running."""
        if self._process is not None:
            return self._process.poll() is None
        # Check if something else is running on the port
        return self._is_port_in_use()

    def _find_binary(self) -> Optional[str]:
        """
        Find the sixtyseven binary.

        Search order:
        1. Explicit binary_path if provided
        2. SIXTYSEVEN_BINARY environment variable
        3. Bundled binary inside the Python package
        4. System PATH
        5. Common installation locations
        """
        # Check explicit path
        if self.binary_path:
            if os.path.isfile(self.binary_path) and os.access(
                self.binary_path, os.X_OK
            ):
                return self.binary_path
            raise ServerError(
                f"Specified binary not found or not executable: {self.binary_path}"
            )

        # Check environment variable
        env_binary = os.environ.get("SIXTYSEVEN_BINARY")
        if env_binary:
            if os.path.isfile(env_binary) and os.access(env_binary, os.X_OK):
                return env_binary
            raise ServerError(f"SIXTYSEVEN_BINARY points to invalid path: {env_binary}")

        # Check bundled binary in the package
        bundled_binary = self._bundled_binary_path()
        if bundled_binary:
            return bundled_binary

        # Check system PATH
        binary_name = (
            f"{self.BINARY_NAME}.exe"
            if platform.system() == "Windows"
            else self.BINARY_NAME
        )
        path_binary = shutil.which(binary_name)
        if path_binary:
            return path_binary

        # Check current working directory (for local development)
        cwd_binary = os.path.join(os.getcwd(), binary_name)
        if os.path.isfile(cwd_binary) and os.access(cwd_binary, os.X_OK):
            return cwd_binary

        # Check common installation locations
        common_paths = self._get_common_paths()
        for path in common_paths:
            if os.path.isfile(path) and os.access(path, os.X_OK):
                return path

        return None

    def _bundled_binary_path(self) -> Optional[str]:
        """Return the path to a bundled binary inside the package, if present."""
        platform_id = self._platform_id()
        if not platform_id:
            return None

        binary_name = (
            f"{self.BINARY_NAME}.exe"
            if platform.system() == "Windows"
            else self.BINARY_NAME
        )
        base_dir = Path(__file__).resolve().parent
        candidate = base_dir / "bin" / platform_id / binary_name
        if candidate.is_file() and os.access(candidate, os.X_OK):
            return str(candidate)
        return None

    def _platform_id(self) -> Optional[str]:
        """Return platform identifier used for bundled binaries."""
        system = platform.system().lower()
        machine = platform.machine().lower()

        if machine in {"x86_64", "amd64"}:
            arch = "amd64"
        elif machine in {"aarch64", "arm64"}:
            arch = "arm64"
        else:
            return None

        if system == "darwin":
            return f"darwin-{arch}"
        if system == "linux":
            return f"linux-{arch}"
        if system == "windows":
            return f"windows-{arch}"
        return None

    def _get_common_paths(self) -> list[str]:
        """Get common installation paths based on platform."""
        system = platform.system()
        home = Path.home()

        if system == "Darwin":
            return [
                "/usr/local/bin/sixtyseven",
                "/opt/homebrew/bin/sixtyseven",
                str(home / ".local" / "bin" / "sixtyseven"),
                str(home / "bin" / "sixtyseven"),
            ]
        elif system == "Windows":
            return [
                str(home / "AppData" / "Local" / "sixtyseven" / "sixtyseven.exe"),
                str(home / "scoop" / "shims" / "sixtyseven.exe"),
                "C:\\Program Files\\sixtyseven\\sixtyseven.exe",
            ]
        else:  # Linux
            return [
                "/usr/local/bin/sixtyseven",
                "/usr/bin/sixtyseven",
                str(home / ".local" / "bin" / "sixtyseven"),
                str(home / "bin" / "sixtyseven"),
            ]

    def _is_port_in_use(self) -> bool:
        """Check if something is actually listening on the port."""
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
            s.settimeout(1)
            try:
                s.connect((self.host, self.port))
                return True  # Connection succeeded, something is listening
            except (OSError, ConnectionRefusedError):
                return False  # Nothing listening

    def _wait_for_health(self) -> bool:
        """Wait for the server to respond to health checks."""
        import urllib.request
        import urllib.error

        health_url = f"{self.url}/health"
        start_time = time.time()

        while time.time() - start_time < self.HEALTH_CHECK_TIMEOUT:
            # Check if process died
            if self._process.poll() is not None:
                return False

            try:
                req = urllib.request.Request(health_url, method="GET")
                with urllib.request.urlopen(req, timeout=1) as response:
                    if response.status == 200:
                        return True
            except (urllib.error.URLError, OSError):
                pass

            time.sleep(self.HEALTH_CHECK_INTERVAL)

        return False

    def __del__(self):
        """Cleanup on garbage collection."""
        if not self.keep_running and self._we_started_server:
            self.stop()


# Global server instance for standalone use
_global_server: Optional[ServerManager] = None


def start_server(
    logdir: Optional[str] = None,
    port: int = ServerManager.DEFAULT_PORT,
    host: str = ServerManager.DEFAULT_HOST,
    open_browser: bool = True,
) -> str:
    """
    Start the Sixtyseven viewer server.

    This starts a server that persists across multiple training runs.
    Unlike `Run(start_server=True)`, the server won't stop when a run ends.

    The server is started once and reused - calling this multiple times
    is safe and will just return the existing server URL.

    Args:
        logdir: Directory containing logs. Defaults to SIXTYSEVEN_LOGDIR or ~/.sixtyseven/logs
        port: Port to run on (default: 6767)
        host: Host to bind to (default: localhost)
        open_browser: Whether to open browser on first start (default: True)

    Returns:
        The server URL (e.g., "http://localhost:6767")

    Example:
        import sixtyseven

        # Start server once at the beginning
        sixtyseven.start_server()

        # Run multiple training stages - server stays alive
        with sixtyseven.Run(project="my-project") as run:
            ...

        resumed = sixtyseven.resume(run.id, config={...})
        with resumed:
            ...

        # Server is still running for viewing results
    """
    global _global_server

    # Determine logdir
    if logdir is None:
        logdir = os.environ.get("SIXTYSEVEN_LOGDIR", "~/.sixtyseven/logs")

    # Expand ~ in path
    logdir = os.path.expanduser(logdir)

    # If server already exists and matches config, just return URL
    if _global_server is not None and _global_server.is_running:
        return _global_server.url

    # Create and start new server
    _global_server = ServerManager(
        logdir=logdir,
        port=port,
        host=host,
        open_browser=open_browser,
        keep_running=True,  # Don't stop on cleanup
    )

    return _global_server.start()


def stop_server() -> None:
    """
    Stop the global Sixtyseven viewer server.

    This is optional - the server will keep running until the Python
    process exits, which is usually what you want for viewing results.
    """
    global _global_server

    if _global_server is not None:
        _global_server.keep_running = False  # Allow stopping
        _global_server.stop()
        _global_server = None


def get_server_url() -> Optional[str]:
    """
    Get the URL of the running server, if any.

    Returns:
        Server URL or None if no server is running
    """
    global _global_server

    if _global_server is not None and _global_server.is_running:
        return _global_server.url
    return None
