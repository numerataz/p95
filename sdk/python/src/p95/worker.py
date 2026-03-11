"""Worker for distributed job execution.

Example usage:
    from p95 import Worker

    worker = Worker(project="team/app", tags=["gpu", "a100"])
    worker.run()

Or via CLI:
    p95 worker start --project team/app --tags gpu,a100
"""

from __future__ import annotations

import logging
import os
import platform
import signal
import subprocess
import sys
import time
import uuid
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional

from p95.client import P95Client
from p95.config import SDKConfig, get_config
from p95.exceptions import APIError


logger = logging.getLogger(__name__)


@dataclass
class WorkerCapabilities:
    """Worker hardware capabilities."""
    gpu_count: int = 0
    gpu_memory_gb: float = 0.0
    gpu_model: str = ""
    cpu_count: int = 0
    memory_gb: float = 0.0
    disk_gb: float = 0.0


@dataclass
class Job:
    """Represents a job to be executed."""
    id: str
    type: str
    status: str
    script: Optional[str] = None
    command: Optional[str] = None
    config: Dict[str, Any] = field(default_factory=dict)
    environment: Dict[str, str] = field(default_factory=dict)
    ai_rationale: Optional[str] = None


class Worker:
    """Distributed worker that claims and executes jobs.

    The worker connects to the p95 API, registers itself, claims available
    jobs, and executes them. It sends heartbeats to indicate it's alive
    and reports job completion/failure.
    """

    def __init__(
        self,
        project: str,
        worker_id: Optional[str] = None,
        tags: Optional[List[str]] = None,
        capabilities: Optional[WorkerCapabilities] = None,
        heartbeat_interval: int = 30,
        poll_interval: int = 5,
    ):
        """Initialize the worker.

        Args:
            project: Project in format 'team/app'
            worker_id: Unique worker ID (auto-generated if not provided)
            tags: Worker tags for capability matching
            capabilities: Hardware capabilities (auto-detected if not provided)
            heartbeat_interval: Seconds between heartbeats
            poll_interval: Seconds between job polling
        """
        self.project = project
        parts = project.split("/")
        if len(parts) != 2:
            raise ValueError(f"Invalid project format: {project}. Expected 'team/app'")
        self.team_slug, self.app_slug = parts

        self.worker_id = worker_id or self._generate_worker_id()
        self.tags = tags or []
        self.capabilities = capabilities or self._detect_capabilities()
        self.heartbeat_interval = heartbeat_interval
        self.poll_interval = poll_interval

        self._running = False
        self._current_job: Optional[Job] = None
        self._client = self._create_client()

    def _generate_worker_id(self) -> str:
        """Generate a unique worker ID."""
        hostname = platform.node() or "unknown"
        short_uuid = str(uuid.uuid4())[:8]
        return f"{hostname}-{short_uuid}"

    def _detect_capabilities(self) -> WorkerCapabilities:
        """Auto-detect hardware capabilities."""
        import os

        caps = WorkerCapabilities()

        # CPU count
        try:
            caps.cpu_count = os.cpu_count() or 1
        except Exception:
            caps.cpu_count = 1

        # Memory (basic detection)
        try:
            if sys.platform == "darwin":
                import subprocess
                result = subprocess.run(
                    ["sysctl", "-n", "hw.memsize"],
                    capture_output=True, text=True
                )
                caps.memory_gb = int(result.stdout.strip()) / (1024 ** 3)
            elif sys.platform == "linux":
                with open("/proc/meminfo") as f:
                    for line in f:
                        if line.startswith("MemTotal:"):
                            # Value is in kB
                            caps.memory_gb = int(line.split()[1]) / (1024 ** 2)
                            break
        except Exception:
            pass

        # GPU detection (NVIDIA only for now)
        try:
            result = subprocess.run(
                ["nvidia-smi", "--query-gpu=count,memory.total,name", "--format=csv,noheader,nounits"],
                capture_output=True, text=True
            )
            if result.returncode == 0:
                lines = result.stdout.strip().split("\n")
                caps.gpu_count = len(lines)
                if lines:
                    parts = lines[0].split(",")
                    if len(parts) >= 2:
                        caps.gpu_memory_gb = float(parts[1].strip()) / 1024
                    if len(parts) >= 3:
                        caps.gpu_model = parts[2].strip()
        except Exception:
            pass

        return caps

    def _create_client(self) -> P95Client:
        """Create the API client."""
        config = get_config()
        if not config.api_key:
            api_key = os.environ.get("P95_API_KEY")
            if not api_key:
                raise ValueError("P95_API_KEY environment variable is required")
            config.api_key = api_key
        if not config.base_url:
            config.base_url = os.environ.get("P95_URL", "https://api.p95.dev")
        return P95Client(config)

    def _register(self) -> None:
        """Register the worker with the API."""
        data = {
            "id": self.worker_id,
            "capabilities": {
                "gpu_count": self.capabilities.gpu_count,
                "gpu_memory_gb": self.capabilities.gpu_memory_gb,
                "gpu_model": self.capabilities.gpu_model,
                "cpu_count": self.capabilities.cpu_count,
                "memory_gb": self.capabilities.memory_gb,
            },
            "tags": self.tags,
            "hostname": platform.node(),
            "system_info": {
                "os": platform.system(),
                "arch": platform.machine(),
                "python": platform.python_version(),
            },
        }

        try:
            self._client._request(
                "POST",
                f"/teams/{self.team_slug}/apps/{self.app_slug}/workers",
                data=data,
            )
            logger.info(f"Worker {self.worker_id} registered successfully")
        except APIError as e:
            logger.error(f"Failed to register worker: {e}")
            raise

    def _heartbeat(self) -> None:
        """Send a heartbeat to indicate the worker is alive."""
        try:
            status = "busy" if self._current_job else "online"
            data = {"status": status}
            if self._current_job:
                data["current_job_id"] = self._current_job.id

            self._client._request(
                "POST",
                f"/workers/{self.worker_id}/heartbeat",
                data=data,
            )
        except APIError as e:
            logger.warning(f"Heartbeat failed: {e}")

    def _claim_job(self) -> Optional[Job]:
        """Try to claim an available job."""
        try:
            # Get available jobs
            response = self._client._request(
                "GET",
                f"/workers/{self.worker_id}/jobs",
                params={"limit": 1},
            )

            jobs = response.get("jobs", [])
            if not jobs:
                return None

            job_data = jobs[0]
            job_id = job_data["id"]

            # Try to claim it
            response = self._client._request(
                "POST",
                f"/jobs/{job_id}/claim",
                data={"worker_id": self.worker_id},
            )

            return Job(
                id=response["id"],
                type=response.get("type", "training"),
                status=response.get("status", "queued"),
                script=response.get("script"),
                command=response.get("command"),
                config=response.get("config", {}),
                environment=response.get("environment", {}),
                ai_rationale=response.get("ai_rationale"),
            )
        except APIError as e:
            if "not available" in str(e).lower():
                # Race condition - another worker claimed it
                return None
            logger.error(f"Failed to claim job: {e}")
            return None

    def _execute_job(self, job: Job) -> int:
        """Execute a job and return exit code."""
        logger.info(f"Executing job {job.id} (type: {job.type})")

        if job.ai_rationale:
            logger.info(f"AI Rationale: {job.ai_rationale}")

        # Notify job started
        try:
            self._client._request(
                "PUT",
                f"/jobs/{job.id}/status",
                data={"status": "running"},
            )
        except APIError:
            pass

        # Build environment
        env = os.environ.copy()
        env.update(job.environment or {})
        env["P95_JOB_ID"] = job.id
        env["P95_PROJECT"] = self.project

        # Add config as environment variables
        for key, value in (job.config or {}).items():
            env[f"P95_CONFIG_{key.upper()}"] = str(value)

        try:
            if job.command:
                # Run command directly
                result = subprocess.run(
                    job.command,
                    shell=True,
                    env=env,
                    capture_output=False,
                )
                return result.returncode
            elif job.script:
                # Write script to temp file and run
                import tempfile
                with tempfile.NamedTemporaryFile(
                    mode="w", suffix=".py", delete=False
                ) as f:
                    f.write(job.script)
                    script_path = f.name

                try:
                    result = subprocess.run(
                        [sys.executable, script_path],
                        env=env,
                        capture_output=False,
                    )
                    return result.returncode
                finally:
                    os.unlink(script_path)
            else:
                logger.error("Job has no command or script")
                return 1
        except Exception as e:
            logger.error(f"Job execution failed: {e}")
            return 1

    def _report_completion(self, job: Job, exit_code: int) -> None:
        """Report job completion or failure."""
        try:
            if exit_code == 0:
                self._client._request(
                    "POST",
                    f"/jobs/{job.id}/complete",
                    data={"worker_id": self.worker_id, "exit_code": exit_code},
                )
                logger.info(f"Job {job.id} completed successfully")
            else:
                self._client._request(
                    "POST",
                    f"/jobs/{job.id}/fail",
                    data={
                        "worker_id": self.worker_id,
                        "error_message": f"Exit code: {exit_code}",
                    },
                )
                logger.info(f"Job {job.id} failed with exit code {exit_code}")
        except APIError as e:
            logger.error(f"Failed to report job completion: {e}")

    def _unregister(self) -> None:
        """Unregister the worker."""
        try:
            self._client._request("DELETE", f"/workers/{self.worker_id}")
            logger.info(f"Worker {self.worker_id} unregistered")
        except APIError as e:
            logger.warning(f"Failed to unregister worker: {e}")

    def run(self) -> None:
        """Run the worker loop.

        The worker will:
        1. Register with the API
        2. Poll for available jobs
        3. Claim and execute jobs
        4. Send heartbeats
        5. Report job completion/failure

        Handles SIGINT/SIGTERM for graceful shutdown.
        """
        self._running = True

        # Set up signal handlers
        def handle_signal(signum, frame):
            logger.info("Received shutdown signal")
            self._running = False

        signal.signal(signal.SIGINT, handle_signal)
        signal.signal(signal.SIGTERM, handle_signal)

        logger.info(f"Starting worker {self.worker_id}")
        logger.info(f"Project: {self.project}")
        logger.info(f"Tags: {self.tags}")
        logger.info(f"Capabilities: GPU={self.capabilities.gpu_count}, "
                    f"CPU={self.capabilities.cpu_count}, "
                    f"Memory={self.capabilities.memory_gb:.1f}GB")

        # Register
        self._register()

        last_heartbeat = 0.0
        last_poll = 0.0

        try:
            while self._running:
                now = time.time()

                # Send heartbeat if needed
                if now - last_heartbeat >= self.heartbeat_interval:
                    self._heartbeat()
                    last_heartbeat = now

                # Poll for jobs if not currently executing
                if self._current_job is None and now - last_poll >= self.poll_interval:
                    job = self._claim_job()
                    if job:
                        self._current_job = job
                        exit_code = self._execute_job(job)
                        self._report_completion(job, exit_code)
                        self._current_job = None
                    last_poll = now

                # Sleep briefly to avoid busy-waiting
                time.sleep(1)

        finally:
            self._unregister()
            logger.info("Worker stopped")


def start_worker(
    project: str,
    worker_id: Optional[str] = None,
    tags: Optional[List[str]] = None,
) -> Worker:
    """Start a worker and return it.

    Args:
        project: Project in format 'team/app'
        worker_id: Unique worker ID (auto-generated if not provided)
        tags: Worker tags for capability matching

    Returns:
        The running Worker instance
    """
    worker = Worker(project=project, worker_id=worker_id, tags=tags)
    worker.run()
    return worker
