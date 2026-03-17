"""Utility functions for the p95 SDK."""

import os
import platform
import subprocess
from typing import Any, Dict, List, Optional


def get_git_info() -> Optional[Dict[str, Any]]:
    """
    Capture git repository information.

    Returns:
        Dictionary with commit, branch, remote, dirty status, and message.
        None if not in a git repository.
    """
    try:
        # Check if in git repo
        subprocess.run(
            ["git", "rev-parse", "--git-dir"],
            capture_output=True,
            check=True,
            timeout=5,
        )

        # Get commit hash
        commit = subprocess.run(
            ["git", "rev-parse", "HEAD"],
            capture_output=True,
            text=True,
            timeout=5,
        ).stdout.strip()

        # Get branch name
        branch = subprocess.run(
            ["git", "rev-parse", "--abbrev-ref", "HEAD"],
            capture_output=True,
            text=True,
            timeout=5,
        ).stdout.strip()

        # Get remote URL
        remote = subprocess.run(
            ["git", "config", "--get", "remote.origin.url"],
            capture_output=True,
            text=True,
            timeout=5,
        ).stdout.strip()

        # Check for uncommitted changes
        status = subprocess.run(
            ["git", "status", "--porcelain"],
            capture_output=True,
            text=True,
            timeout=5,
        )
        dirty = bool(status.stdout.strip())

        # Get commit message
        message = subprocess.run(
            ["git", "log", "-1", "--pretty=%B"],
            capture_output=True,
            text=True,
            timeout=5,
        ).stdout.strip()

        return {
            "commit": commit,
            "branch": branch,
            "remote": remote,
            "dirty": dirty,
            "message": message[:200] if message else None,  # Truncate long messages
        }

    except (
        subprocess.CalledProcessError,
        subprocess.TimeoutExpired,
        FileNotFoundError,
    ):
        return None


def get_system_info() -> Dict[str, Any]:
    """
    Capture system and hardware information.

    Returns:
        Dictionary with hostname, OS, Python version, CPU, memory, and GPU info.
    """
    import sys

    info = {
        "hostname": platform.node(),
        "os": f"{platform.system()} {platform.release()}",
        "python_version": f"{sys.version_info.major}.{sys.version_info.minor}.{sys.version_info.micro}",
        "cpu_count": os.cpu_count(),
    }

    # Try to get memory info
    try:
        import psutil

        mem = psutil.virtual_memory()
        info["memory_gb"] = round(mem.total / (1024**3), 2)
    except ImportError:
        pass

    # Try to get GPU info
    gpu_info = get_gpu_info()
    if gpu_info:
        info["gpu_info"] = gpu_info

    return info


def get_gpu_info() -> Optional[List[str]]:
    """
    Get GPU information using nvidia-smi.

    Returns:
        List of GPU names, or None if no NVIDIA GPUs found.
    """
    try:
        result = subprocess.run(
            ["nvidia-smi", "--query-gpu=name", "--format=csv,noheader"],
            capture_output=True,
            text=True,
            timeout=10,
        )

        if result.returncode == 0:
            gpus = [
                line.strip()
                for line in result.stdout.strip().split("\n")
                if line.strip()
            ]
            return gpus if gpus else None

    except (
        subprocess.CalledProcessError,
        subprocess.TimeoutExpired,
        FileNotFoundError,
    ):
        pass

    # Try PyTorch
    try:
        import torch

        if torch.cuda.is_available():
            return [
                torch.cuda.get_device_name(i) for i in range(torch.cuda.device_count())
            ]
    except ImportError:
        pass

    return None


def generate_run_name() -> str:
    """Generate a unique run name."""
    import time
    from uuid import uuid4

    adjectives = ["swift", "bright", "calm", "bold", "keen", "wise", "pure", "warm"]
    nouns = ["falcon", "river", "forest", "peak", "star", "wave", "cloud", "dawn"]

    adj = adjectives[int(time.time() * 1000) % len(adjectives)]
    noun = nouns[int(time.time() * 1000 // 7) % len(nouns)]

    return f"{adj}-{noun}-{uuid4().hex[:8]}"
