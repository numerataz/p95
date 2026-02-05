#!/usr/bin/env python3
"""
Demo script for Sixtyseven local mode.

This demonstrates logging metrics to local files that can be viewed
with the Sixtyseven local viewer.

Usage:
    # Just run this script - the viewer starts automatically!
    python examples/local_mode_demo.py

    # Or start the viewer manually in a separate terminal:
    sixtyseven --logdir ./logs
"""

import os
import sys
import math
import time
import random

# Add the SDK to the path for development
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "../sdk/python/src"))

# Set the log directory (optional - uses default if not set)
os.environ["SIXTYSEVEN_LOGDIR"] = "./logs"

from sixtyseven import Run


def simulate_training():
    """Simulate a training loop with metrics."""

    print("Starting simulated training run...")
    print(
        f"Logs will be written to: {os.environ.get('SIXTYSEVEN_LOGDIR', '~/.sixtyseven/logs')}"
    )
    print()

    with Run(
        project="demo-project-3",
        name=f"training-{int(time.time())}",
        tags=["demo", "local-mode"],
        config={
            "learning_rate": 0.001,
            "batch_size": 32,
            "optimizer": "adam",
            "epochs": 50,
        },
        start_server=True,  # Automatically start viewer and open browser
    ) as run:
        print(f"Run ID: {run.id}")
        print(f"Run directory: {run.logdir}")
        print()

        # Simulate 500 epochs for longer streaming
        for epoch in range(500):
            # Simulate decreasing loss with some noise
            base_loss = 1.0 * math.exp(-0.01 * epoch)
            train_loss = base_loss + random.gauss(0, 0.03)
            val_loss = base_loss * 1.1 + random.gauss(0, 0.05)

            # Simulate increasing accuracy
            train_acc = min(
                0.99,
                0.5 + 0.45 * (1 - math.exp(-0.02 * epoch)) + random.gauss(0, 0.015),
            )
            val_acc = min(0.98, train_acc * 0.95 + random.gauss(0, 0.02))

            # Log metrics
            run.log_metrics(
                {
                    "train/loss": max(0, train_loss),
                    "train/accuracy": max(0, min(1, train_acc)),
                    "val/loss": max(0, val_loss),
                    "val/accuracy": max(0, min(1, val_acc)),
                    "learning_rate": 0.001 * (0.95 ** (epoch // 50)),
                },
                step=epoch,
            )

            # Print progress
            if epoch % 50 == 0:
                print(
                    f"Epoch {epoch:3d}: train_loss={train_loss:.4f}, val_acc={val_acc:.4f}"
                )

            # Simulate training time (faster iterations)
            time.sleep(0.05)

        print()
        print("Training complete!")


if __name__ == "__main__":
    simulate_training()
