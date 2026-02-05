#!/usr/bin/env python3
"""
Simple training simulation with Sixtyseven metric tracking.

This example doesn't require PyTorch - it simulates a training loop
to demonstrate the sixtyseven SDK.

Usage:
    # First, start the sixtyseven server:
    # cd deployments/docker && docker-compose up -d

    # Install the SDK:
    # pip install -e sdk/python

    # Set your API key:
    # export SIXTYSEVEN_API_KEY=ss67_your_key_here

    # Run:
    # python examples/simple_training.py
"""

import math
import random
import time
import os

from sixtyseven import Run, configure

# Configure sixtyseven
configure(
    base_url=os.environ.get("SIXTYSEVEN_URL", "http://localhost:8080"),
)


def simulate_training_step(epoch: int, step: int, base_loss: float) -> dict:
    """Simulate a training step with realistic-looking metrics."""
    # Simulate loss decreasing over time with some noise
    decay = math.exp(-epoch * 0.3 - step * 0.001)
    noise = random.gauss(0, 0.02)
    loss = base_loss * decay + noise + 0.1

    # Accuracy increases as loss decreases
    accuracy = 1.0 - (loss * 0.8) + random.gauss(0, 0.01)
    accuracy = max(0, min(1, accuracy))  # Clamp to [0, 1]

    return {
        "loss": max(0.01, loss),
        "accuracy": accuracy,
    }


def main():
    # Hyperparameters
    config = {
        "learning_rate": 0.001,
        "batch_size": 32,
        "epochs": 100,  # Longer for live testing
        "hidden_layers": [128, 64],
        "dropout": 0.2,
        "optimizer": "adam",
        "dataset": "synthetic",
    }

    print("=" * 50)
    print("Sixtyseven Training Example")
    print("=" * 50)

    # Start a run with sixtyseven
    # NOTE: You need to create the team and app first via the API
    with Run(
        project="default/ninetyfive",  # Format: team-slug/app-slug
        name="training-run-01",
        tags=["example", "simulation"],
        config=config,
    ) as run:
        print(f"\nRun ID: {run.id}")
        print(f"Project: {run.project}")
        print("View metrics in the TUI: make run-tui")
        print()

        base_loss = 2.5
        steps_per_epoch = 100
        best_val_accuracy = 0

        for epoch in range(config["epochs"]):
            print(f"\nEpoch {epoch + 1}/{config['epochs']}")
            print("-" * 30)

            epoch_loss = 0
            epoch_accuracy = 0

            # Training loop
            for step in range(steps_per_epoch):
                # Simulate training step
                metrics = simulate_training_step(epoch, step, base_loss)

                epoch_loss += metrics["loss"]
                epoch_accuracy += metrics["accuracy"]

                # Calculate global step
                global_step = epoch * steps_per_epoch + step

                # Log every 10 steps
                if step % 10 == 0:
                    run.log_metrics(
                        {
                            "train/loss": metrics["loss"],
                            "train/accuracy": metrics["accuracy"],
                        },
                        step=global_step,
                    )

                # Simulate training time
                time.sleep(0.02)

            # Calculate epoch averages
            avg_loss = epoch_loss / steps_per_epoch
            avg_accuracy = epoch_accuracy / steps_per_epoch

            # Simulate validation
            val_loss = avg_loss * (1 + random.gauss(0, 0.1))
            val_accuracy = avg_accuracy * (1 + random.gauss(0, 0.02))
            val_accuracy = max(0, min(1, val_accuracy))

            # Log epoch metrics
            run.log_metrics(
                {
                    "train/epoch_loss": avg_loss,
                    "train/epoch_accuracy": avg_accuracy,
                    "val/loss": val_loss,
                    "val/accuracy": val_accuracy,
                    "epoch": epoch + 1,
                },
                step=(epoch + 1) * steps_per_epoch,
            )

            # Track best validation accuracy
            if val_accuracy > best_val_accuracy:
                best_val_accuracy = val_accuracy
                run.log_metrics(
                    {
                        "best_val_accuracy": best_val_accuracy,
                    },
                    step=(epoch + 1) * steps_per_epoch,
                )

            print(f"Train Loss: {avg_loss:.4f} | Train Acc: {avg_accuracy:.4f}")
            print(f"Val Loss:   {val_loss:.4f} | Val Acc:   {val_accuracy:.4f}")

        # Log final results
        run.log_config(
            {
                "final_train_loss": avg_loss,
                "final_val_accuracy": val_accuracy,
                "best_val_accuracy": best_val_accuracy,
            }
        )

        print()
        print("=" * 50)
        print("Training complete!")
        print(f"Best validation accuracy: {best_val_accuracy:.4f}")
        print("=" * 50)


if __name__ == "__main__":
    main()
