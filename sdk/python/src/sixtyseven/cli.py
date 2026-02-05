"""Console entrypoint for the bundled sixtyseven CLI."""

from __future__ import annotations

import os
import platform
import shutil
import sys
from pathlib import Path
from typing import Optional


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

    binary_name = "sixtyseven.exe" if platform.system() == "Windows" else "sixtyseven"
    base_dir = Path(__file__).resolve().parent
    candidate = base_dir / "bin" / platform_id / binary_name
    if candidate.is_file() and os.access(candidate, os.X_OK):
        return str(candidate)
    return None


def _find_binary() -> Optional[str]:
    bundled = _bundled_binary_path()
    if bundled:
        return bundled

    binary_name = "sixtyseven.exe" if platform.system() == "Windows" else "sixtyseven"
    return shutil.which(binary_name)


def main() -> None:
    binary = _find_binary()
    if not binary:
        print(
            "Could not find 'sixtyseven' binary. Reinstall the package or set SIXTYSEVEN_BINARY.",
            file=sys.stderr,
        )
        raise SystemExit(1)

    os.execv(binary, [binary, *sys.argv[1:]])
