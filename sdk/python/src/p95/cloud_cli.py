"""Cloud CLI commands for p95 AI-driven ML training.

This module provides CLI commands for interacting with the p95 cloud API,
enabling Claude Code and other AI agents to query runs, create jobs,
and intervene in running experiments.
"""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import tempfile
from typing import Any, Dict, Optional

from p95.client import P95Client
from p95.config import get_config


def _get_client() -> P95Client:
    """Get a configured P95 cloud client."""
    config = get_config()
    if not config.api_key:
        api_key = os.environ.get("P95_API_KEY")
        if not api_key:
            _error("P95_API_KEY environment variable is required")
        config.api_key = api_key
    if not config.base_url:
        config.base_url = os.environ.get("P95_URL", "https://api.p95.dev")
    return P95Client(config)


def _output(data: Any, as_json: bool = True) -> None:
    """Output data in JSON or human-readable format."""
    if as_json:
        print(json.dumps({"success": True, "data": data, "error": None}, indent=2))
    else:
        print(data)


def _error(message: str, as_json: bool = True) -> None:
    """Output error message and exit."""
    if as_json:
        print(json.dumps({"success": False, "data": None, "error": message}))
    else:
        print(f"Error: {message}", file=sys.stderr)
    sys.exit(1)


def _parse_project(project: str) -> tuple[str, str]:
    """Parse project string into team and app slugs."""
    parts = project.split("/")
    if len(parts) != 2:
        _error(f"Invalid project format: {project}. Expected 'team/app'")
    return parts[0], parts[1]


# ===========================================
# Local Execution Helpers
# ===========================================


def _install_requirements(requirements: str, env: dict) -> tuple[bool, str]:
    """Install Python requirements using uv (fast) or pip (fallback).

    Returns:
        Tuple of (success, log_output)
    """
    # Parse requirements: "torch,transformers>=4.0" -> ["torch", "transformers>=4.0"]
    reqs = [r.strip() for r in requirements.split(",") if r.strip()]
    if not reqs:
        return True, ""

    print(f"Installing requirements: {reqs}", file=sys.stderr)

    # Try uv first (much faster), fall back to pip
    for installer in ["uv pip install", f"{sys.executable} -m pip install"]:
        try:
            cmd = installer.split() + reqs
            result = subprocess.run(
                cmd,
                env=env,
                capture_output=True,
                text=True,
                timeout=300,  # 5 minute timeout for installs
            )
            output = result.stdout + result.stderr
            if result.returncode == 0:
                print(
                    f"Requirements installed with {installer.split()[0]}",
                    file=sys.stderr,
                )
                return True, output
        except FileNotFoundError:
            continue  # Try next installer
        except subprocess.TimeoutExpired:
            return False, "Requirement installation timed out after 5 minutes"
        except Exception:
            continue

    return False, "Failed to install requirements with both uv and pip"


def _execute_job_locally(
    job_id: str,
    project: str,
    script: Optional[str],
    command: Optional[str],
    config: Dict[str, Any],
    python_requirements: Optional[str],
) -> tuple[int, str]:
    """Execute a job locally and return (exit_code, logs)."""
    logs = []

    # Build environment
    env = os.environ.copy()
    env["P95_JOB_ID"] = job_id
    env["P95_PROJECT"] = project

    # Add config as environment variables
    for key, value in (config or {}).items():
        env[f"P95_CONFIG_{key.upper()}"] = str(value)

    # Install requirements if specified
    if python_requirements:
        success, install_logs = _install_requirements(python_requirements, env)
        logs.append(f"=== Installing requirements ===\n{install_logs}")
        if not success:
            logs.append(f"\nFailed to install requirements: {python_requirements}")
            return 1, "\n".join(logs)

    try:
        if command:
            # Run command directly
            logs.append(f"=== Running command ===\n$ {command}\n")
            result = subprocess.run(
                command,
                shell=True,
                env=env,
                capture_output=True,
                text=True,
            )
            logs.append(result.stdout)
            if result.stderr:
                logs.append(f"\n=== stderr ===\n{result.stderr}")
            return result.returncode, "\n".join(logs)
        elif script:
            # Write script to temp file and run
            with tempfile.NamedTemporaryFile(mode="w", suffix=".py", delete=False) as f:
                f.write(script)
                script_path = f.name

            logs.append("=== Running script ===\n")
            try:
                result = subprocess.run(
                    [sys.executable, script_path],
                    env=env,
                    capture_output=True,
                    text=True,
                )
                logs.append(result.stdout)
                if result.stderr:
                    logs.append(f"\n=== stderr ===\n{result.stderr}")
                return result.returncode, "\n".join(logs)
            finally:
                os.unlink(script_path)
        else:
            return 1, "Job has no command or script"
    except Exception as e:
        return 1, f"Job execution failed: {e}"


# ===========================================
# Runs Commands
# ===========================================


def runs_list(args: argparse.Namespace) -> None:
    """List runs in a project."""
    client = _get_client()
    team_slug, app_slug = _parse_project(args.project)

    params: Dict[str, Any] = {"limit": args.limit}
    if args.status:
        params["status"] = args.status
    if args.offset:
        params["offset"] = args.offset

    try:
        response = client._request(
            "GET",
            f"/teams/{team_slug}/apps/{app_slug}/runs",
            params=params,
        )
        _output(response, args.json)
    except Exception as e:
        _error(str(e), args.json)


def runs_get(args: argparse.Namespace) -> None:
    """Get run details by ID."""
    client = _get_client()

    try:
        response = client._request("GET", f"/runs/{args.run_id}")
        _output(response, args.json)
    except Exception as e:
        _error(str(e), args.json)


def runs_metrics(args: argparse.Namespace) -> None:
    """Get metrics for a run."""
    client = _get_client()

    try:
        if args.metric_name:
            # Get specific metric series
            params: Dict[str, Any] = {}
            if args.since_step is not None:
                params["min_step"] = args.since_step
            response = client._request(
                "GET",
                f"/runs/{args.run_id}/metrics/{args.metric_name}",
                params=params if params else None,
            )
        else:
            # Get all metrics summary
            response = client._request("GET", f"/runs/{args.run_id}/metrics/latest")
        _output(response, args.json)
    except Exception as e:
        _error(str(e), args.json)


def runs_intervene(args: argparse.Namespace) -> None:
    """Create an intervention for a running experiment."""
    client = _get_client()

    data: Dict[str, Any] = {
        "type": args.action,
        "rationale": args.rationale,
    }

    if args.config:
        try:
            data["config_delta"] = json.loads(args.config)
        except json.JSONDecodeError as e:
            _error(f"Invalid JSON in --config: {e}", args.json)

    if args.step is not None:
        data["step"] = args.step

    try:
        response = client._request(
            "POST",
            f"/runs/{args.run_id}/intervene",
            data=data,
        )
        _output(response, args.json)
    except Exception as e:
        _error(str(e), args.json)


def runs_interventions(args: argparse.Namespace) -> None:
    """List interventions for a run."""
    client = _get_client()

    try:
        response = client._request(
            "GET",
            f"/runs/{args.run_id}/interventions",
            params={"limit": args.limit},
        )
        _output(response, args.json)
    except Exception as e:
        _error(str(e), args.json)


# ===========================================
# Jobs Commands
# ===========================================


def jobs_create(args: argparse.Namespace) -> None:
    """Create a new job."""
    client = _get_client()
    team_slug, app_slug = _parse_project(args.project)

    data: Dict[str, Any] = {
        "type": args.type or "training",
    }

    script_content: Optional[str] = None
    if args.script:
        # Read script content from file
        try:
            with open(args.script, "r") as f:
                script_content = f.read()
                data["script"] = script_content
        except FileNotFoundError:
            _error(f"Script file not found: {args.script}", args.json)

    if args.command:
        data["command"] = args.command

    config: Dict[str, Any] = {}
    if args.config:
        try:
            config = json.loads(args.config)
            data["config"] = config
        except json.JSONDecodeError as e:
            _error(f"Invalid JSON in --config: {e}", args.json)

    if args.requirements:
        data["python_requirements"] = args.requirements

    if args.rationale:
        data["ai_rationale"] = args.rationale
        data["created_by"] = "ai:claude"

    if args.priority is not None:
        data["priority"] = args.priority

    # If --now flag is set, tell the API to create with status=running
    # so workers don't try to claim it
    if getattr(args, "now", False):
        data["run_locally"] = True

    try:
        # Create the job
        response = client._request(
            "POST",
            f"/teams/{team_slug}/apps/{app_slug}/jobs",
            data=data,
        )
        job_id = response["id"]

        # If --now flag is set, execute the job immediately
        if getattr(args, "now", False):
            print(f"Executing job {job_id} locally...", file=sys.stderr)

            # Execute locally (no need to claim - we'll update status directly)
            exit_code, logs = _execute_job_locally(
                job_id=job_id,
                project=args.project,
                script=script_content,
                command=args.command,
                config=config,
                python_requirements=args.requirements,
            )

            # Print logs in real-time feel
            if logs:
                print(logs, file=sys.stderr)

            # Report completion using the local endpoint (no worker required)
            try:
                # Truncate logs if too large (max 1MB)
                max_log_size = 1024 * 1024
                if len(logs) > max_log_size:
                    logs = logs[:max_log_size] + "\n... [truncated]"

                client._request(
                    "POST",
                    f"/jobs/{job_id}/complete-local",
                    data={
                        "exit_code": exit_code,
                        "logs": logs,
                    },
                )
            except Exception as e:
                print(f"Warning: Could not report completion: {e}", file=sys.stderr)

            # Fetch updated job with run_id
            try:
                response = client._request("GET", f"/jobs/{job_id}")
            except Exception:
                pass

            # Add execution info to response
            response["_executed_locally"] = True
            response["_exit_code"] = exit_code

        _output(response, args.json)
    except Exception as e:
        _error(str(e), args.json)


def jobs_get(args: argparse.Namespace) -> None:
    """Get job details by ID."""
    client = _get_client()

    try:
        response = client._request("GET", f"/jobs/{args.job_id}")
        _output(response, args.json)
    except Exception as e:
        _error(str(e), args.json)


def jobs_list(args: argparse.Namespace) -> None:
    """List jobs in a project."""
    client = _get_client()
    team_slug, app_slug = _parse_project(args.project)

    params: Dict[str, Any] = {"limit": args.limit}
    if args.status:
        params["status"] = args.status

    try:
        response = client._request(
            "GET",
            f"/teams/{team_slug}/apps/{app_slug}/jobs",
            params=params,
        )
        _output(response, args.json)
    except Exception as e:
        _error(str(e), args.json)


def jobs_cancel(args: argparse.Namespace) -> None:
    """Cancel a job."""
    client = _get_client()

    try:
        response = client._request("POST", f"/jobs/{args.job_id}/cancel")
        _output(response, args.json)
    except Exception as e:
        _error(str(e), args.json)


# ===========================================
# Workers Commands
# ===========================================


def workers_list(args: argparse.Namespace) -> None:
    """List workers in a project."""
    client = _get_client()
    team_slug, app_slug = _parse_project(args.project)

    params: Dict[str, Any] = {"limit": args.limit}
    if args.status:
        params["status"] = args.status

    try:
        response = client._request(
            "GET",
            f"/teams/{team_slug}/apps/{app_slug}/workers",
            params=params,
        )
        _output(response, args.json)
    except Exception as e:
        _error(str(e), args.json)


def workers_start(args: argparse.Namespace) -> None:
    """Start a worker (runs the worker loop)."""
    from p95.worker import Worker

    try:
        tags = args.tags.split(",") if args.tags else []
        worker = Worker(
            project=args.project,
            tags=tags,
        )
        worker.run()
    except KeyboardInterrupt:
        print("\nWorker stopped.")
    except Exception as e:
        _error(str(e), args.json)


# ===========================================
# Main CLI Entry Point
# ===========================================


def create_parser() -> argparse.ArgumentParser:
    """Create the argument parser for cloud CLI commands."""
    parser = argparse.ArgumentParser(
        prog="p95",
        description="p95 ML experiment tracking CLI",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        default=True,
        help="Output in JSON format (default)",
    )

    subparsers = parser.add_subparsers(dest="command", help="Available commands")

    # -----------------------------------------
    # runs commands
    # -----------------------------------------
    runs_parser = subparsers.add_parser("runs", help="Run operations")
    runs_sub = runs_parser.add_subparsers(dest="runs_command")

    # runs list
    runs_list_parser = runs_sub.add_parser("list", help="List runs")
    runs_list_parser.add_argument(
        "--project", "-p", required=True, help="Project in format 'team/app'"
    )
    runs_list_parser.add_argument("--status", "-s", help="Filter by status")
    runs_list_parser.add_argument("--limit", type=int, default=20)
    runs_list_parser.add_argument("--offset", type=int, default=0)
    runs_list_parser.add_argument("--json", action="store_true", default=True)
    runs_list_parser.set_defaults(func=runs_list)

    # runs get
    runs_get_parser = runs_sub.add_parser("get", help="Get run details")
    runs_get_parser.add_argument("run_id", help="Run ID")
    runs_get_parser.add_argument("--json", action="store_true", default=True)
    runs_get_parser.set_defaults(func=runs_get)

    # runs metrics
    runs_metrics_parser = runs_sub.add_parser("metrics", help="Get run metrics")
    runs_metrics_parser.add_argument("run_id", help="Run ID")
    runs_metrics_parser.add_argument(
        "metric_name", nargs="?", help="Specific metric name"
    )
    runs_metrics_parser.add_argument(
        "--since-step", type=int, help="Get metrics since step"
    )
    runs_metrics_parser.add_argument("--json", action="store_true", default=True)
    runs_metrics_parser.set_defaults(func=runs_metrics)

    # runs intervene
    runs_intervene_parser = runs_sub.add_parser(
        "intervene", help="Create an intervention"
    )
    runs_intervene_parser.add_argument("run_id", help="Run ID")
    runs_intervene_parser.add_argument(
        "--action",
        "-a",
        required=True,
        choices=["adjust_config", "early_stop", "pause", "resume"],
        help="Intervention action",
    )
    runs_intervene_parser.add_argument("--config", "-c", help="Config changes as JSON")
    runs_intervene_parser.add_argument(
        "--rationale", "-r", required=True, help="Explanation for the intervention"
    )
    runs_intervene_parser.add_argument("--step", type=int, help="Current step")
    runs_intervene_parser.add_argument("--json", action="store_true", default=True)
    runs_intervene_parser.set_defaults(func=runs_intervene)

    # runs interventions
    runs_interventions_parser = runs_sub.add_parser(
        "interventions", help="List interventions"
    )
    runs_interventions_parser.add_argument("run_id", help="Run ID")
    runs_interventions_parser.add_argument("--limit", type=int, default=50)
    runs_interventions_parser.add_argument("--json", action="store_true", default=True)
    runs_interventions_parser.set_defaults(func=runs_interventions)

    # -----------------------------------------
    # jobs commands
    # -----------------------------------------
    jobs_parser = subparsers.add_parser("jobs", help="Job queue operations")
    jobs_sub = jobs_parser.add_subparsers(dest="jobs_command")

    # jobs create
    jobs_create_parser = jobs_sub.add_parser("create", help="Create a job")
    jobs_create_parser.add_argument(
        "--project", "-p", required=True, help="Project in format 'team/app'"
    )
    jobs_create_parser.add_argument("--script", help="Path to Python script")
    jobs_create_parser.add_argument("--command", help="Command to execute")
    jobs_create_parser.add_argument("--config", "-c", help="Config as JSON")
    jobs_create_parser.add_argument(
        "--requirements",
        help="Python packages to install (e.g., 'torch,transformers>=4.0')",
    )
    jobs_create_parser.add_argument(
        "--type",
        "-t",
        default="training",
        choices=["training", "sweep_trial", "evaluation", "custom"],
    )
    jobs_create_parser.add_argument(
        "--rationale", "-r", help="AI rationale for creating this job"
    )
    jobs_create_parser.add_argument("--priority", type=int, default=0)
    jobs_create_parser.add_argument(
        "--now", action="store_true", help="Execute the job immediately (locally)"
    )
    jobs_create_parser.add_argument("--json", action="store_true", default=True)
    jobs_create_parser.set_defaults(func=jobs_create)

    # jobs get
    jobs_get_parser = jobs_sub.add_parser("get", help="Get job details")
    jobs_get_parser.add_argument("job_id", help="Job ID")
    jobs_get_parser.add_argument("--json", action="store_true", default=True)
    jobs_get_parser.set_defaults(func=jobs_get)

    # jobs list
    jobs_list_parser = jobs_sub.add_parser("list", help="List jobs")
    jobs_list_parser.add_argument(
        "--project", "-p", required=True, help="Project in format 'team/app'"
    )
    jobs_list_parser.add_argument("--status", "-s", help="Filter by status")
    jobs_list_parser.add_argument("--limit", type=int, default=20)
    jobs_list_parser.add_argument("--json", action="store_true", default=True)
    jobs_list_parser.set_defaults(func=jobs_list)

    # jobs cancel
    jobs_cancel_parser = jobs_sub.add_parser("cancel", help="Cancel a job")
    jobs_cancel_parser.add_argument("job_id", help="Job ID")
    jobs_cancel_parser.add_argument("--json", action="store_true", default=True)
    jobs_cancel_parser.set_defaults(func=jobs_cancel)

    # -----------------------------------------
    # workers commands
    # -----------------------------------------
    workers_parser = subparsers.add_parser("workers", help="Worker operations")
    workers_sub = workers_parser.add_subparsers(dest="workers_command")

    # workers list
    workers_list_parser = workers_sub.add_parser("list", help="List workers")
    workers_list_parser.add_argument(
        "--project", "-p", required=True, help="Project in format 'team/app'"
    )
    workers_list_parser.add_argument("--status", "-s", help="Filter by status")
    workers_list_parser.add_argument("--limit", type=int, default=50)
    workers_list_parser.add_argument("--json", action="store_true", default=True)
    workers_list_parser.set_defaults(func=workers_list)

    # -----------------------------------------
    # worker command (singular - starts a worker)
    # -----------------------------------------
    worker_parser = subparsers.add_parser("worker", help="Worker daemon")
    worker_sub = worker_parser.add_subparsers(dest="worker_command")

    worker_start_parser = worker_sub.add_parser("start", help="Start a worker")
    worker_start_parser.add_argument(
        "--project", "-p", required=True, help="Project in format 'team/app'"
    )
    worker_start_parser.add_argument("--tags", help="Comma-separated worker tags")
    worker_start_parser.add_argument("--json", action="store_true", default=True)
    worker_start_parser.set_defaults(func=workers_start)

    return parser


def main_cloud() -> None:
    """Main entry point for cloud CLI commands."""
    parser = create_parser()
    args = parser.parse_args()

    if not hasattr(args, "func"):
        parser.print_help()
        sys.exit(1)

    args.func(args)


if __name__ == "__main__":
    main_cloud()
