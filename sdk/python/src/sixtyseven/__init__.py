"""
Sixtyseven - ML Experiment Tracking SDK

A Python SDK for tracking machine learning experiments with Sixtyseven.

Example usage:
    from sixtyseven import Run

    with Run(project="my-team/image-classifier") as run:
        run.log_config({"learning_rate": 0.001})

        for epoch in range(100):
            loss = train()
            run.log_metrics({"loss": loss}, step=epoch)

Resuming a run:
    from sixtyseven import resume

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

from sixtyseven.config import configure
from sixtyseven.exceptions import (
    APIError,
    AuthenticationError,
    ServerError,
    SixtySevenError,
    ValidationError,
)
from sixtyseven.run import Run, resume
from sixtyseven.server import start_server, stop_server

__version__ = "0.1.0"
__all__ = [
    "Run",
    "resume",
    "configure",
    "start_server",
    "stop_server",
    "SixtySevenError",
    "AuthenticationError",
    "APIError",
    "ValidationError",
    "ServerError",
]
