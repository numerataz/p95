"""Console entrypoint for the p95 CLI.

Supports both local mode (Go binary) and cloud mode (Python commands).
Cloud commands: jobs, workers, runs (with cloud API), intervene
Local commands: tui, ls, show, serve
"""

from __future__ import annotations

import os
import platform
import shutil
import sys
from pathlib import Path
from typing import Optional


# Cloud commands that are handled by Python
CLOUD_COMMANDS = {"jobs", "workers", "worker", "runs", "run"}


def _platform_id() -> Optional[str]:
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


def _bundled_binary_path() -> Optional[str]:
    platform_id = _platform_id()
    if not platform_id:
        return None

    binary_name = "pnf.exe" if platform.system() == "Windows" else "pnf"
    base_dir = Path(__file__).resolve().parent
    candidate = base_dir / "bin" / platform_id / binary_name
    if candidate.is_file() and os.access(candidate, os.X_OK):
        return str(candidate)
    return None


def _find_binary() -> Optional[str]:
    bundled = _bundled_binary_path()
    if bundled:
        return bundled

    binary_name = "pnf.exe" if platform.system() == "Windows" else "pnf"
    return shutil.which(binary_name)


def _is_cloud_command() -> bool:
    """Check if the command should be handled by cloud CLI."""
    if len(sys.argv) < 2:
        return False

    cmd = sys.argv[1]
    return cmd in CLOUD_COMMANDS


def main() -> None:
    """Main CLI entry point.

    Routes to cloud CLI for cloud commands (jobs, workers, runs intervene),
    or to the Go binary for local commands (tui, ls, show, serve).
    """
    # Check for cloud commands first
    if _is_cloud_command():
        from p95.cloud_cli import main_cloud
        main_cloud()
        return

    # Fall back to Go binary for local commands
    binary = _find_binary()
    if not binary:
        print(
            "Could not find 'pnf' binary. Reinstall the package or set P95_BINARY.",
            file=sys.stderr,
        )
        raise SystemExit(1)

    os.execv(binary, [binary, *sys.argv[1:]])
