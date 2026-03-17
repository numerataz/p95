"""Tests for p95 cloud CLI commands."""

import json
import os

import pytest

from p95.cloud_cli import (
    _execute_job_locally,
    _install_requirements,
    _parse_project,
    create_parser,
)


class TestParseProject:
    """Tests for project string parsing."""

    def test_valid_project(self):
        """Test parsing a valid project string."""
        team, app = _parse_project("my-team/my-app")
        assert team == "my-team"
        assert app == "my-app"

    def test_project_with_hyphens(self):
        """Test parsing a project with hyphens."""
        team, app = _parse_project("team-name-123/app-name-456")
        assert team == "team-name-123"
        assert app == "app-name-456"

    def test_invalid_project_no_slash(self):
        """Test that invalid project format raises error."""
        with pytest.raises(SystemExit):
            _parse_project("invalid-format")

    def test_invalid_project_too_many_slashes(self):
        """Test that too many slashes raises error."""
        with pytest.raises(SystemExit):
            _parse_project("a/b/c")


class TestCreateParser:
    """Tests for CLI argument parser."""

    def test_parser_creation(self):
        """Test that parser is created successfully."""
        parser = create_parser()
        assert parser is not None
        assert parser.prog == "p95"

    def test_jobs_create_args(self):
        """Test jobs create command arguments."""
        parser = create_parser()
        args = parser.parse_args(
            [
                "jobs",
                "create",
                "--project",
                "team/app",
                "--script",
                "train.py",
                "--requirements",
                "numpy,torch",
                "--config",
                '{"lr": 0.001}',
                "--rationale",
                "Testing something",
                "--now",
            ]
        )

        assert args.project == "team/app"
        assert args.script == "train.py"
        assert args.requirements == "numpy,torch"
        assert args.config == '{"lr": 0.001}'
        assert args.rationale == "Testing something"
        assert args.now is True

    def test_jobs_create_without_now(self):
        """Test jobs create without --now flag."""
        parser = create_parser()
        args = parser.parse_args(
            [
                "jobs",
                "create",
                "--project",
                "team/app",
                "--script",
                "train.py",
            ]
        )

        assert args.now is False

    def test_jobs_list_args(self):
        """Test jobs list command arguments."""
        parser = create_parser()
        args = parser.parse_args(
            [
                "jobs",
                "list",
                "--project",
                "team/app",
                "--status",
                "pending",
                "--limit",
                "50",
            ]
        )

        assert args.project == "team/app"
        assert args.status == "pending"
        assert args.limit == 50

    def test_jobs_get_args(self):
        """Test jobs get command arguments."""
        parser = create_parser()
        args = parser.parse_args(
            [
                "jobs",
                "get",
                "abc123-job-id",
            ]
        )

        assert args.job_id == "abc123-job-id"

    def test_worker_start_args(self):
        """Test worker start command arguments."""
        parser = create_parser()
        args = parser.parse_args(
            [
                "worker",
                "start",
                "--project",
                "team/app",
                "--tags",
                "gpu,a100",
            ]
        )

        assert args.project == "team/app"
        assert args.tags == "gpu,a100"


class TestInstallRequirements:
    """Tests for requirements installation."""

    def test_empty_requirements(self):
        """Test with empty requirements string."""
        success, logs = _install_requirements("", os.environ.copy())
        assert success is True
        assert logs == ""

    def test_whitespace_requirements(self):
        """Test with whitespace-only requirements."""
        success, logs = _install_requirements("  ,  ,  ", os.environ.copy())
        assert success is True
        assert logs == ""

    def test_parse_requirements(self):
        """Test requirements parsing."""
        # This tests the parsing logic without actually installing
        requirements = "numpy,torch>=2.0,transformers"
        reqs = [r.strip() for r in requirements.split(",") if r.strip()]

        assert len(reqs) == 3
        assert reqs[0] == "numpy"
        assert reqs[1] == "torch>=2.0"
        assert reqs[2] == "transformers"


class TestExecuteJobLocally:
    """Tests for local job execution."""

    def test_execute_script(self):
        """Test executing a simple script."""
        script = "print('Hello, World!')"
        exit_code, logs = _execute_job_locally(
            job_id="test-123",
            project="team/app",
            script=script,
            command=None,
            config={},
            python_requirements=None,
        )

        assert exit_code == 0
        assert "Hello, World!" in logs

    def test_execute_script_with_config(self):
        """Test executing a script with config environment variables."""
        script = """
import os
print(f"EPOCHS={os.environ.get('P95_CONFIG_EPOCHS', 'not set')}")
print(f"LR={os.environ.get('P95_CONFIG_LR', 'not set')}")
"""
        exit_code, logs = _execute_job_locally(
            job_id="test-123",
            project="team/app",
            script=script,
            command=None,
            config={"epochs": 10, "lr": 0.001},
            python_requirements=None,
        )

        assert exit_code == 0
        assert "EPOCHS=10" in logs
        assert "LR=0.001" in logs

    def test_execute_script_failure(self):
        """Test executing a script that fails."""
        script = "raise Exception('Test error')"
        exit_code, logs = _execute_job_locally(
            job_id="test-123",
            project="team/app",
            script=script,
            command=None,
            config={},
            python_requirements=None,
        )

        assert exit_code != 0
        assert "Test error" in logs or "Exception" in logs

    def test_execute_command(self):
        """Test executing a command."""
        exit_code, logs = _execute_job_locally(
            job_id="test-123",
            project="team/app",
            script=None,
            command="echo 'Command executed'",
            config={},
            python_requirements=None,
        )

        assert exit_code == 0
        assert "Command executed" in logs

    def test_execute_no_script_or_command(self):
        """Test that missing script and command returns error."""
        exit_code, logs = _execute_job_locally(
            job_id="test-123",
            project="team/app",
            script=None,
            command=None,
            config={},
            python_requirements=None,
        )

        assert exit_code == 1
        assert "no command or script" in logs.lower()

    def test_job_id_in_environment(self):
        """Test that job ID is available in environment."""
        script = """
import os
print(f"JOB_ID={os.environ.get('P95_JOB_ID', 'not set')}")
"""
        exit_code, logs = _execute_job_locally(
            job_id="test-job-12345",
            project="team/app",
            script=script,
            command=None,
            config={},
            python_requirements=None,
        )

        assert exit_code == 0
        assert "JOB_ID=test-job-12345" in logs

    def test_project_in_environment(self):
        """Test that project is available in environment."""
        script = """
import os
print(f"PROJECT={os.environ.get('P95_PROJECT', 'not set')}")
"""
        exit_code, logs = _execute_job_locally(
            job_id="test-123",
            project="my-team/my-app",
            script=script,
            command=None,
            config={},
            python_requirements=None,
        )

        assert exit_code == 0
        assert "PROJECT=my-team/my-app" in logs


class TestJobCreateData:
    """Tests for job create data construction."""

    def test_run_locally_flag_in_data(self):
        """Test that run_locally flag is set when --now is used."""
        parser = create_parser()
        args = parser.parse_args(
            [
                "jobs",
                "create",
                "--project",
                "team/app",
                "--script",
                "train.py",
                "--now",
            ]
        )

        # Simulate the data construction logic from jobs_create
        data = {"type": "training"}
        if getattr(args, "now", False):
            data["run_locally"] = True

        assert data["run_locally"] is True

    def test_no_run_locally_without_now(self):
        """Test that run_locally is not set without --now."""
        parser = create_parser()
        args = parser.parse_args(
            [
                "jobs",
                "create",
                "--project",
                "team/app",
                "--script",
                "train.py",
            ]
        )

        data = {"type": "training"}
        if getattr(args, "now", False):
            data["run_locally"] = True

        assert "run_locally" not in data


class TestWorkerTags:
    """Tests for worker tag parsing."""

    def test_single_tag(self):
        """Test parsing a single tag."""
        parser = create_parser()
        args = parser.parse_args(
            [
                "worker",
                "start",
                "--project",
                "team/app",
                "--tags",
                "gpu",
            ]
        )

        tags = args.tags.split(",") if args.tags else []
        assert tags == ["gpu"]

    def test_multiple_tags(self):
        """Test parsing multiple tags."""
        parser = create_parser()
        args = parser.parse_args(
            [
                "worker",
                "start",
                "--project",
                "team/app",
                "--tags",
                "gpu,a100,high-memory",
            ]
        )

        tags = args.tags.split(",") if args.tags else []
        assert tags == ["gpu", "a100", "high-memory"]

    def test_no_tags(self):
        """Test worker start without tags."""
        parser = create_parser()
        args = parser.parse_args(
            [
                "worker",
                "start",
                "--project",
                "team/app",
            ]
        )

        tags = args.tags.split(",") if args.tags else []
        assert tags == []


class TestConfigParsing:
    """Tests for config JSON parsing."""

    def test_valid_config(self):
        """Test parsing valid JSON config."""
        config_str = '{"epochs": 10, "lr": 0.001, "batch_size": 32}'
        config = json.loads(config_str)

        assert config["epochs"] == 10
        assert config["lr"] == 0.001
        assert config["batch_size"] == 32

    def test_nested_config(self):
        """Test parsing nested JSON config."""
        config_str = '{"model": {"hidden_size": 128}, "training": {"epochs": 10}}'
        config = json.loads(config_str)

        assert config["model"]["hidden_size"] == 128
        assert config["training"]["epochs"] == 10

    def test_invalid_config(self):
        """Test that invalid JSON raises error."""
        config_str = "not valid json"
        with pytest.raises(json.JSONDecodeError):
            json.loads(config_str)


class TestLogsTruncation:
    """Tests for log truncation logic."""

    def test_short_logs_not_truncated(self):
        """Test that short logs are not truncated."""
        logs = "Short log message"
        max_log_size = 1024 * 1024  # 1MB

        if len(logs) > max_log_size:
            logs = logs[:max_log_size] + "\n... [truncated]"

        assert logs == "Short log message"
        assert "[truncated]" not in logs

    def test_long_logs_truncated(self):
        """Test that long logs are truncated."""
        logs = "x" * (2 * 1024 * 1024)  # 2MB
        max_log_size = 1024 * 1024  # 1MB

        if len(logs) > max_log_size:
            logs = logs[:max_log_size] + "\n... [truncated]"

        assert len(logs) < 2 * 1024 * 1024
        assert "[truncated]" in logs
