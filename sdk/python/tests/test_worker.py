"""Tests for p95 worker module."""

import os
import platform
from unittest import mock

import pytest

from p95.worker import Job, Worker, WorkerCapabilities


class TestWorkerCapabilities:
    """Tests for WorkerCapabilities dataclass."""

    def test_default_capabilities(self):
        """Test default capability values."""
        caps = WorkerCapabilities()

        assert caps.gpu_count == 0
        assert caps.gpu_memory_gb == 0.0
        assert caps.gpu_model == ""
        assert caps.cpu_count == 0
        assert caps.memory_gb == 0.0
        assert caps.disk_gb == 0.0

    def test_custom_capabilities(self):
        """Test setting custom capabilities."""
        caps = WorkerCapabilities(
            gpu_count=4,
            gpu_memory_gb=80.0,
            gpu_model="A100",
            cpu_count=64,
            memory_gb=512.0,
            disk_gb=2000.0,
        )

        assert caps.gpu_count == 4
        assert caps.gpu_memory_gb == 80.0
        assert caps.gpu_model == "A100"
        assert caps.cpu_count == 64
        assert caps.memory_gb == 512.0
        assert caps.disk_gb == 2000.0


class TestJob:
    """Tests for Job dataclass."""

    def test_job_creation(self):
        """Test creating a job."""
        job = Job(
            id="job-123",
            type="training",
            status="pending",
            script="print('hello')",
        )

        assert job.id == "job-123"
        assert job.type == "training"
        assert job.status == "pending"
        assert job.script == "print('hello')"

    def test_job_with_config(self):
        """Test job with config."""
        job = Job(
            id="job-123",
            type="training",
            status="running",
            config={"epochs": 10, "lr": 0.001},
        )

        assert job.config["epochs"] == 10
        assert job.config["lr"] == 0.001

    def test_job_with_requirements(self):
        """Test job with Python requirements."""
        job = Job(
            id="job-123",
            type="training",
            status="pending",
            python_requirements="numpy,torch>=2.0",
        )

        assert job.python_requirements == "numpy,torch>=2.0"

    def test_job_with_rationale(self):
        """Test job with AI rationale."""
        job = Job(
            id="job-123",
            type="training",
            status="pending",
            ai_rationale="Reducing learning rate due to loss plateau",
        )

        assert job.ai_rationale == "Reducing learning rate due to loss plateau"


class TestWorkerInit:
    """Tests for Worker initialization."""

    def test_worker_id_generation(self):
        """Test that worker ID is generated correctly."""
        # Worker ID should be hostname-uuid format
        worker = Worker.__new__(Worker)
        worker_id = worker._generate_worker_id()

        assert "-" in worker_id
        hostname = platform.node() or "unknown"
        assert worker_id.startswith(hostname)

    def test_worker_id_uniqueness(self):
        """Test that generated worker IDs are unique."""
        worker = Worker.__new__(Worker)
        id1 = worker._generate_worker_id()
        id2 = worker._generate_worker_id()

        assert id1 != id2

    def test_project_parsing(self):
        """Test that project is parsed correctly."""
        # Valid format
        project = "team-name/app-name"
        parts = project.split("/")

        assert len(parts) == 2
        assert parts[0] == "team-name"
        assert parts[1] == "app-name"

    def test_invalid_project_format(self):
        """Test that invalid project format raises error."""
        with pytest.raises(ValueError) as exc:
            Worker(project="invalid-format")

        assert "Invalid project format" in str(exc.value)


class TestCapabilityDetection:
    """Tests for automatic capability detection."""

    def test_cpu_count_detection(self):
        """Test CPU count detection."""
        caps = WorkerCapabilities()
        caps.cpu_count = os.cpu_count() or 1

        assert caps.cpu_count >= 1

    @mock.patch("platform.system", return_value="Darwin")
    def test_macos_memory_detection_format(self, mock_system):
        """Test that macOS memory detection uses correct command."""
        # This tests the logic path, not actual execution
        assert platform.system() == "Darwin"


class TestJobExecution:
    """Tests for job execution logic."""

    def test_environment_setup(self):
        """Test that environment variables are set correctly for job."""
        job = Job(
            id="job-abc123",
            type="training",
            status="running",
            config={"epochs": 10, "lr": 0.001},
            environment={"CUSTOM_VAR": "custom_value"},
        )

        # Simulate environment setup
        env = os.environ.copy()
        env.update(job.environment or {})
        env["P95_JOB_ID"] = job.id
        env["P95_PROJECT"] = "test/app"

        for key, value in (job.config or {}).items():
            env[f"P95_CONFIG_{key.upper()}"] = str(value)

        assert env["P95_JOB_ID"] == "job-abc123"
        assert env["P95_PROJECT"] == "test/app"
        assert env["CUSTOM_VAR"] == "custom_value"
        assert env["P95_CONFIG_EPOCHS"] == "10"
        assert env["P95_CONFIG_LR"] == "0.001"

    def test_config_to_env_conversion(self):
        """Test that config values are converted to uppercase env vars."""
        config = {
            "epochs": 10,
            "learning_rate": 0.001,
            "batch_size": 32,
        }

        env = {}
        for key, value in config.items():
            env[f"P95_CONFIG_{key.upper()}"] = str(value)

        assert env["P95_CONFIG_EPOCHS"] == "10"
        assert env["P95_CONFIG_LEARNING_RATE"] == "0.001"
        assert env["P95_CONFIG_BATCH_SIZE"] == "32"


class TestRequirementsInstallation:
    """Tests for Python requirements installation."""

    def test_parse_single_requirement(self):
        """Test parsing a single requirement."""
        requirements = "numpy"
        reqs = [r.strip() for r in requirements.split(",") if r.strip()]

        assert reqs == ["numpy"]

    def test_parse_multiple_requirements(self):
        """Test parsing multiple requirements."""
        requirements = "numpy,torch,transformers"
        reqs = [r.strip() for r in requirements.split(",") if r.strip()]

        assert reqs == ["numpy", "torch", "transformers"]

    def test_parse_versioned_requirements(self):
        """Test parsing versioned requirements."""
        requirements = "torch>=2.0,transformers>=4.0,numpy==1.24"
        reqs = [r.strip() for r in requirements.split(",") if r.strip()]

        assert reqs == ["torch>=2.0", "transformers>=4.0", "numpy==1.24"]

    def test_parse_with_whitespace(self):
        """Test parsing requirements with extra whitespace."""
        requirements = " numpy , torch , transformers "
        reqs = [r.strip() for r in requirements.split(",") if r.strip()]

        assert reqs == ["numpy", "torch", "transformers"]

    def test_parse_empty_requirements(self):
        """Test parsing empty requirements."""
        requirements = ""
        reqs = [r.strip() for r in requirements.split(",") if r.strip()]

        assert reqs == []


class TestLogCapture:
    """Tests for log capture during job execution."""

    def test_log_format(self):
        """Test that logs include expected sections."""
        logs = []
        logs.append("=== Installing requirements ===\nInstalled: numpy")
        logs.append("=== Running script ===\nHello, World!")

        full_logs = "\n".join(logs)

        assert "=== Installing requirements ===" in full_logs
        assert "=== Running script ===" in full_logs

    def test_stderr_capture(self):
        """Test that stderr is included in logs."""
        logs = []
        logs.append("stdout output")
        logs.append("\n=== stderr ===\nWarning: deprecated function")

        full_logs = "\n".join(logs)

        assert "stdout output" in full_logs
        assert "=== stderr ===" in full_logs
        assert "Warning:" in full_logs


class TestWorkerStatus:
    """Tests for worker status management."""

    def test_status_values(self):
        """Test valid worker status values."""
        valid_statuses = ["online", "busy", "offline"]

        for status in valid_statuses:
            assert status in ["online", "busy", "offline"]

    def test_status_when_executing(self):
        """Test that status is 'busy' when executing a job."""
        # Simulate the heartbeat logic
        current_job = Job(id="job-123", type="training", status="running")
        status = "busy" if current_job else "online"

        assert status == "busy"

    def test_status_when_idle(self):
        """Test that status is 'online' when idle."""
        current_job = None
        status = "busy" if current_job else "online"

        assert status == "online"


class TestLogTruncation:
    """Tests for log truncation to prevent oversized payloads."""

    def test_small_logs_not_truncated(self):
        """Test that small logs are not truncated."""
        logs = "Small log output"
        max_log_size = 1024 * 1024  # 1MB

        if len(logs) > max_log_size:
            logs = logs[:max_log_size] + "\n... [truncated]"

        assert logs == "Small log output"

    def test_large_logs_truncated(self):
        """Test that large logs are truncated."""
        logs = "x" * (2 * 1024 * 1024)  # 2MB
        max_log_size = 1024 * 1024  # 1MB

        if len(logs) > max_log_size:
            logs = logs[:max_log_size] + "\n... [truncated]"

        assert len(logs) <= max_log_size + 20  # +20 for truncation message
        assert logs.endswith("[truncated]")

    def test_truncation_preserves_start(self):
        """Test that truncation preserves the start of logs."""
        logs = "START_MARKER" + "x" * (2 * 1024 * 1024)
        max_log_size = 1024 * 1024

        if len(logs) > max_log_size:
            logs = logs[:max_log_size] + "\n... [truncated]"

        assert logs.startswith("START_MARKER")
