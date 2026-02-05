#!/usr/bin/env python3
"""Stage the platform-specific sixtyseven binary into the Python package."""

from __future__ import annotations

import os
import platform
import shutil
import sys
from pathlib import Path


def _platform_id() -> str | None:
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


def main() -> int:
    repo_root = Path(__file__).resolve().parents[3]
    binary_path = os.environ.get("SIXTYSEVEN_BINARY_PATH")
    if binary_path:
        binary = Path(binary_path)
    else:
        binary = repo_root / "sixtyseven"

    if platform.system() == "Windows" and binary.suffix.lower() != ".exe":
        binary = binary.with_suffix(".exe")

    if not binary.is_file():
        print(f"Binary not found: {binary}", file=sys.stderr)
        return 1

    platform_id = _platform_id()
    if not platform_id:
        print("Unsupported platform for staging", file=sys.stderr)
        return 1

    dest_dir = repo_root / "sdk" / "python" / "src" / "sixtyseven" / "bin" / platform_id
    dest_dir.mkdir(parents=True, exist_ok=True)

    dest = dest_dir / binary.name
    shutil.copy2(binary, dest)
    if platform.system() != "Windows":
        dest.chmod(0o755)

    print(f"Staged binary to {dest}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
