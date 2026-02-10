"""
p95 - ML Experiment Tracking SDK

A Python SDK for tracking machine learning experiments with p95.

Example usage:
    from p95 import Run

    with Run(project="my-team/image-classifier") as run:
        run.log_config({"learning_rate": 0.001})

        for epoch in range(100):
            loss = train()
            run.log_metrics({"loss": loss}, step=epoch)

Resuming a run:
    from p95 import resume

    # Resume with new config
    resumed = resume(
        run.id,  # or path to run directory for local mode
        config={"lr": 0.0001},
        note="Reduced LR for fine-tuning"
    )

    with resumed:
        for epoch in range(100, 200):
            resumed.log_metrics({"loss": compute_loss()}, step=epoch)
"""

from p95.config import configure
from p95.exceptions import (
    APIError,
    AuthenticationError,
    ServerError,
    P95Error,
    ValidationError,
)
from p95.run import Run, resume
from p95.server import start_server, stop_server

__version__ = "0.1.0"
__all__ = [
    "Run",
    "resume",
    "configure",
    "start_server",
    "stop_server",
    "P95Error",
    "AuthenticationError",
    "APIError",
    "ValidationError",
    "ServerError",
]
