"""
Hyperparameter sweep functionality for p95.

Example usage:
    import p95
    from p95.sweep import SweepConfig, ParameterSpec

    # Create sweep
    sweep_id = p95.sweep(
        project="team/app",
        config=SweepConfig(
            method="random",
            metric="val_loss",
            goal="minimize",
            parameters=[
                ParameterSpec("lr", "log_uniform", min=1e-5, max=0.1),
                ParameterSpec("batch_size", "categorical", values=[16, 32, 64]),
            ],
            max_runs=20,
            early_stopping={"method": "median", "min_steps": 5, "warmup": 3},
        ),
    )

    # Training function - runs are auto-linked to sweep!
    def train(params):
        # Any Run created here is automatically part of the sweep
        with p95.Run(project="team/app") as run:
            for epoch in range(100):
                loss = train_epoch(lr=params["lr"], batch_size=params["batch_size"])
                run.log_metrics({"val_loss": loss}, step=epoch)

    # Run agent
    p95.agent(sweep_id, train)
"""

import contextvars
import json
import math
import os
import random
import time
from dataclasses import asdict, dataclass, field
from datetime import datetime
from pathlib import Path
from typing import TYPE_CHECKING, Any, Callable, Dict, List, Optional, Union

from p95.client import P95Client
from p95.config import get_config

if TYPE_CHECKING:
    from p95.run import Run

# Thread-local context for current sweep
_current_sweep_context: contextvars.ContextVar[Optional["SweepContext"]] = (
    contextvars.ContextVar("current_sweep_context", default=None)
)


@dataclass
class SweepContext:
    """Context for the currently running sweep agent."""

    sweep_id: str
    params: Dict[str, Any]
    run_index: int
    sweep_data: Dict[str, Any]
    sweep_dir: Optional[Path] = None
    project: Optional[str] = None
    is_local: bool = False


def get_current_sweep_context() -> Optional[SweepContext]:
    """Get the current sweep context if running inside an agent."""
    return _current_sweep_context.get()


def _set_sweep_context(ctx: Optional[SweepContext]) -> contextvars.Token:
    """Set the current sweep context."""
    return _current_sweep_context.set(ctx)


@dataclass
class ParameterSpec:
    """Specification for a hyperparameter to search."""

    name: str
    type: str  # uniform, log_uniform, int, categorical
    min: Optional[float] = None
    max: Optional[float] = None
    values: Optional[List[Any]] = None

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for serialization."""
        result = {"name": self.name, "type": self.type}
        if self.min is not None:
            result["min"] = self.min
        if self.max is not None:
            result["max"] = self.max
        if self.values is not None:
            result["values"] = self.values
        return result


@dataclass
class SweepConfig:
    """Configuration for a hyperparameter sweep."""

    method: str  # "random" or "grid"
    metric: str  # e.g., "val_loss"
    goal: str  # "minimize" or "maximize"
    parameters: List[ParameterSpec]
    config: Optional[Dict[str, Any]] = None  # Static config passed to all runs
    max_runs: Optional[int] = None
    early_stopping: Optional[Dict[str, Any]] = None

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for API serialization."""
        result = {
            "method": self.method,
            "metric_name": self.metric,
            "metric_goal": self.goal,
            "search_space": {"parameters": [p.to_dict() for p in self.parameters]},
        }
        if self.config:
            result["config"] = self.config
        if self.max_runs is not None:
            result["max_runs"] = self.max_runs
        if self.early_stopping:
            result["early_stopping"] = self.early_stopping
        return result


def sweep(
    project: str,
    config: SweepConfig,
    name: Optional[str] = None,
) -> str:
    """
    Create a hyperparameter sweep.

    Args:
        project: Project identifier ("team/app" for remote, "my-project" for local)
        config: Sweep configuration
        name: Optional sweep name

    Returns:
        Sweep ID
    """
    sdk_config = get_config()

    # Check if remote or local mode
    if "/" in project:
        # Remote mode: team/app format
        return _create_remote_sweep(sdk_config, project, config, name)
    else:
        # Local mode
        return _create_local_sweep(project, config, name)


def should_prune(
    run: "Run",
    metric_name: str,
    value: float,
    step: int,
) -> bool:
    """
    Check if a run should be pruned based on early stopping.

    Call this during training after logging the target metric to check
    if the run should be stopped early.

    Args:
        run: The current Run object (must be part of a sweep)
        metric_name: The metric name to check
        value: The current metric value
        step: The current step

    Returns:
        True if the run should be pruned, False otherwise

    Example:
        for epoch in range(100):
            loss = train_epoch()
            run.log_metrics({"val_loss": loss}, step=epoch)

            if p95.should_prune(run, "val_loss", loss, epoch):
                print("Pruning run due to poor performance")
                break
    """
    # Check if this run is part of a sweep
    if not hasattr(run, "_sweep_id") or not run._sweep_id:
        return False

    sweep_id = run._sweep_id

    # Remote sweep
    if not sweep_id.startswith("local:"):
        config = get_config()
        client = P95Client(config)
        try:
            response = client._request(
                "POST",
                f"/sweeps/{sweep_id}/report",
                data={
                    "run_id": run.id,
                    "value": value,
                    "step": step,
                },
            )
            return response.get("should_prune", False)
        except Exception:
            return False

    # Local sweep
    if not hasattr(run, "_sweep_data") or not hasattr(run, "_sweep_dir"):
        return False

    sweep_data = run._sweep_data
    sweep_dir = run._sweep_dir

    # Check early stopping config
    early_stopping = sweep_data.get("early_stopping")
    if not early_stopping:
        return False

    min_steps = early_stopping.get("min_steps", 0)
    warmup = early_stopping.get("warmup", 0)

    if step < min_steps:
        return False

    if sweep_data.get("run_count", 0) < warmup:
        return False

    # Get median at this step
    run_id = (
        run._local_writer.run_id
        if hasattr(run, "_local_writer") and run._local_writer
        else None
    )
    median = _get_median_at_step_local(
        sweep_dir,
        run._project,
        metric_name,
        step,
        run_id or "",
    )

    if median is None:
        return False

    goal = sweep_data.get("metric_goal", "minimize")
    if goal == "minimize":
        return value > median
    else:
        return value < median


def agent(
    sweep_id: str,
    train_fn: Callable[[Dict[str, Any]], None],
    count: Optional[int] = None,
    project: Optional[str] = None,
) -> None:
    """
    Run a sweep agent that executes training runs.

    The agent will:
    1. Request next parameters from the sweep
    2. Create a run with those parameters
    3. Call the training function
    4. Report results and check for pruning
    5. Repeat until sweep is complete or count is reached

    Args:
        sweep_id: The sweep ID
        train_fn: Function that takes params dict and performs training
        count: Maximum number of runs to execute (None for unlimited)
        project: Project for local mode (required for local sweeps)
    """
    sdk_config = get_config()

    # Determine if local or remote
    if sweep_id.startswith("local:"):
        _run_local_agent(sweep_id, train_fn, count, project)
    else:
        _run_remote_agent(sdk_config, sweep_id, train_fn, count)


def _create_remote_sweep(
    sdk_config,
    project: str,
    config: SweepConfig,
    name: Optional[str],
) -> str:
    """Create a sweep on the remote server."""
    client = P95Client(sdk_config)

    parts = project.split("/")
    if len(parts) != 2:
        raise ValueError("Project must be in 'team/app' format for remote mode")

    team_slug, app_slug = parts

    data = config.to_dict()
    if name:
        data["name"] = name

    response = client._request(
        "POST",
        f"/teams/{team_slug}/apps/{app_slug}/sweeps",
        data=data,
    )

    return response["id"]


def _create_local_sweep(
    project: str,
    config: SweepConfig,
    name: Optional[str],
) -> str:
    """Create a sweep in local storage."""
    import uuid

    sweep_id = str(uuid.uuid4())

    # Create sweep directory
    logs_dir = Path.home() / ".p95" / "logs" / project / ".sweeps" / sweep_id
    logs_dir.mkdir(parents=True, exist_ok=True)

    # Write sweep config
    sweep_data = {
        "id": sweep_id,
        "name": name or f"sweep-{sweep_id[:8]}",
        "status": "running",
        "method": config.method,
        "metric_name": config.metric,
        "metric_goal": config.goal,
        "search_space": {"parameters": [p.to_dict() for p in config.parameters]},
        "config": config.config or {},
        "max_runs": config.max_runs,
        "early_stopping": config.early_stopping,
        "run_count": 0,
        "grid_index": 0,
        "runs": [],
        "best_run_id": None,
        "best_value": None,
        "created_at": datetime.now().isoformat(),
    }

    with open(logs_dir / "sweep.json", "w") as f:
        json.dump(sweep_data, f, indent=2)

    return f"local:{project}:{sweep_id}"


def _run_remote_agent(
    sdk_config,
    sweep_id: str,
    train_fn: Callable[[Dict[str, Any]], None],
    count: Optional[int],
) -> None:
    """Run agent against remote server."""
    client = P95Client(sdk_config)
    runs_completed = 0

    while count is None or runs_completed < count:
        # Get next parameters
        response = client._request("GET", f"/sweeps/{sweep_id}/next")

        if response.get("done", False):
            print("Sweep complete")
            break

        params = response.get("params", {})
        run_index = response.get("run_index", runs_completed + 1)

        # Get sweep info for config
        sweep = client._request("GET", f"/sweeps/{sweep_id}")

        print(f"Starting run {run_index} with params: {params}")

        # Set sweep context so any Run created inside train_fn is auto-linked
        ctx = SweepContext(
            sweep_id=sweep_id,
            params=params,
            run_index=run_index,
            sweep_data=sweep,
            is_local=False,
        )
        token = _set_sweep_context(ctx)

        try:
            # Call the training function
            train_fn(params)
        except _PrunedException as e:
            print(f"Run pruned: {e}")
        except Exception as e:
            print(f"Run failed: {e}")
        finally:
            _set_sweep_context(None)

        runs_completed += 1


class _PrunedException(Exception):
    """Raised when a run should be pruned."""

    pass


def _run_local_agent(
    sweep_id: str,
    train_fn: Callable[[Dict[str, Any]], None],
    count: Optional[int],
    project: Optional[str],
) -> None:
    """Run agent for local sweeps."""
    from p95.run import Run

    # Parse sweep ID
    parts = sweep_id.split(":")
    if len(parts) != 3:
        raise ValueError("Invalid local sweep ID format")

    _, proj, sid = parts
    project = project or proj

    sweep_dir = Path.home() / ".p95" / "logs" / project / ".sweeps" / sid
    sweep_file = sweep_dir / "sweep.json"

    if not sweep_file.exists():
        raise ValueError(f"Sweep not found: {sweep_id}")

    runs_completed = 0

    while count is None or runs_completed < count:
        # Load sweep state
        with open(sweep_file) as f:
            sweep_data = json.load(f)

        if sweep_data["status"] != "running":
            print(f"Sweep {sweep_data['status']}")
            break

        # Check max runs
        if sweep_data["max_runs"] and sweep_data["run_count"] >= sweep_data["max_runs"]:
            sweep_data["status"] = "completed"
            with open(sweep_file, "w") as f:
                json.dump(sweep_data, f, indent=2)
            print("Sweep complete (max runs reached)")
            break

        # Get next parameters
        params, done = _get_local_params(sweep_data)

        if done:
            sweep_data["status"] = "completed"
            with open(sweep_file, "w") as f:
                json.dump(sweep_data, f, indent=2)
            print("Sweep complete")
            break

        run_index = sweep_data["run_count"] + 1
        sweep_data["run_count"] = run_index

        # Save updated sweep state
        with open(sweep_file, "w") as f:
            json.dump(sweep_data, f, indent=2)

        print(f"Starting run {run_index} with params: {params}")

        # Set sweep context so any Run created inside train_fn is auto-linked
        ctx = SweepContext(
            sweep_id=sweep_id,
            params=params,
            run_index=run_index,
            sweep_data=sweep_data,
            sweep_dir=sweep_dir,
            project=project,
            is_local=True,
        )
        token = _set_sweep_context(ctx)

        try:
            # Call the training function - it will create its own Run
            # which will automatically pick up the sweep context
            train_fn(params)

            # After training, find the run that was created and update sweep best
            # The Run class will have registered itself in the context
            if hasattr(ctx, "_run") and ctx._run:
                _update_local_sweep_best(sweep_file, ctx._run, sweep_data)

        except _PrunedException as e:
            print(f"Run pruned: {e}")
            if hasattr(ctx, "_run") and ctx._run:
                _update_local_sweep_best(sweep_file, ctx._run, sweep_data)
        except Exception as e:
            print(f"Run failed: {e}")
        finally:
            _set_sweep_context(None)

        runs_completed += 1


def _get_local_params(sweep_data: Dict[str, Any]) -> tuple[Dict[str, Any], bool]:
    """Get next parameters for local sweep."""
    parameters = sweep_data["search_space"]["parameters"]
    method = sweep_data["method"]

    if method == "random":
        return _sample_random(parameters), False
    elif method == "grid":
        return _get_grid_params(sweep_data, parameters)
    else:
        raise ValueError(f"Unknown search method: {method}")


def _sample_random(parameters: List[Dict[str, Any]]) -> Dict[str, Any]:
    """Sample random values for each parameter."""
    result = {}

    for param in parameters:
        name = param["name"]
        ptype = param["type"]

        if ptype == "uniform":
            result[name] = random.uniform(param["min"], param["max"])
        elif ptype == "log_uniform":
            log_min = math.log(param["min"])
            log_max = math.log(param["max"])
            result[name] = math.exp(random.uniform(log_min, log_max))
        elif ptype == "int":
            result[name] = random.randint(int(param["min"]), int(param["max"]))
        elif ptype == "categorical":
            result[name] = random.choice(param["values"])

    return result


def _get_grid_params(
    sweep_data: Dict[str, Any],
    parameters: List[Dict[str, Any]],
) -> tuple[Dict[str, Any], bool]:
    """Get next grid search parameters."""
    combinations = _build_grid_combinations(parameters)

    grid_index = sweep_data.get("grid_index", 0)

    if grid_index >= len(combinations):
        return {}, True

    sweep_data["grid_index"] = grid_index + 1
    return combinations[grid_index], False


def _build_grid_combinations(
    parameters: List[Dict[str, Any]],
) -> List[Dict[str, Any]]:
    """Build cartesian product of all parameter values."""
    if not parameters:
        return [{}]

    # Get values for each parameter
    param_values = []
    for param in parameters:
        ptype = param["type"]
        if ptype == "categorical":
            param_values.append((param["name"], param["values"]))
        elif ptype == "int":
            vals = list(range(int(param["min"]), int(param["max"]) + 1))
            param_values.append((param["name"], vals))
        else:
            # For continuous, use min and max as grid points
            param_values.append((param["name"], [param["min"], param["max"]]))

    # Build cartesian product
    result = [{}]
    for name, values in param_values:
        new_result = []
        for combo in result:
            for val in values:
                new_combo = combo.copy()
                new_combo[name] = val
                new_result.append(new_combo)
        result = new_result

    return result


def _update_local_sweep_best(
    sweep_file: Path,
    run: Run,
    sweep_data: Dict[str, Any],
) -> None:
    """Update the best run for a local sweep."""
    import sqlite3

    # Reload sweep data to get latest
    with open(sweep_file) as f:
        sweep_data = json.load(f)

    metric_name = sweep_data["metric_name"]
    goal = sweep_data["metric_goal"]

    # Get the run directory from the run object
    if hasattr(run, "_local_writer") and run._local_writer:
        run_dir = run._local_writer.run_dir
    else:
        # Fallback: find the latest run directory
        project_dir = Path.home() / ".p95" / "logs" / run._project
        run_dirs = sorted(project_dir.glob("run-*"), key=lambda p: p.stat().st_mtime)
        if not run_dirs:
            return
        run_dir = run_dirs[-1]

    # Read metric from SQLite database
    db_path = run_dir / "run.db"
    if not db_path.exists():
        return

    best_value = None
    try:
        conn = sqlite3.connect(str(db_path))
        cursor = conn.execute(
            "SELECT value FROM metrics WHERE name = ? ORDER BY step DESC LIMIT 1",
            (metric_name,),
        )
        row = cursor.fetchone()
        if row:
            best_value = row[0]
        conn.close()
    except sqlite3.Error:
        return

    if best_value is None:
        return

    # Check if this is better
    current_best = sweep_data.get("best_value")
    is_better = False

    if current_best is None:
        is_better = True
    elif goal == "minimize":
        is_better = best_value < current_best
    else:
        is_better = best_value > current_best

    run_id = (
        run._local_writer.run_id
        if hasattr(run, "_local_writer") and run._local_writer
        else str(run_dir.name)
    )

    if is_better:
        sweep_data["best_value"] = best_value
        sweep_data["best_run_id"] = run_id
        sweep_data["runs"].append(
            {
                "run_id": run_id,
                "value": best_value,
                "is_best": True,
            }
        )
    else:
        sweep_data["runs"].append(
            {
                "run_id": run_id,
                "value": best_value,
                "is_best": False,
            }
        )

    with open(sweep_file, "w") as f:
        json.dump(sweep_data, f, indent=2)


def _get_median_at_step_local(
    sweep_dir: Path,
    project: str,
    metric_name: str,
    step: int,
    exclude_run_id: str,
) -> Optional[float]:
    """Get median metric value at a specific step across local sweep runs."""
    import sqlite3

    sweep_file = sweep_dir / "sweep.json"
    if not sweep_file.exists():
        return None

    with open(sweep_file) as f:
        sweep_data = json.load(f)

    values = []
    project_dir = Path.home() / ".p95" / "logs" / project

    for run_info in sweep_data.get("runs", []):
        run_id = run_info.get("run_id")
        if run_id == exclude_run_id:
            continue

        # Find the run directory
        for run_dir in project_dir.iterdir():
            if not run_dir.is_dir():
                continue

            meta_file = run_dir / "meta.json"
            if not meta_file.exists():
                continue

            with open(meta_file) as f:
                meta = json.load(f)

            if meta.get("id") == run_id:
                db_path = run_dir / "run.db"
                if db_path.exists():
                    try:
                        conn = sqlite3.connect(str(db_path))
                        cursor = conn.execute(
                            "SELECT value FROM metrics WHERE name = ? AND step = ?",
                            (metric_name, step),
                        )
                        row = cursor.fetchone()
                        if row:
                            values.append(row[0])
                        conn.close()
                    except sqlite3.Error:
                        pass
                break

    if not values:
        return None

    # Calculate median
    values.sort()
    n = len(values)
    if n % 2 == 0:
        return (values[n // 2 - 1] + values[n // 2]) / 2
    return values[n // 2]
