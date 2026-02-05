#!/usr/bin/env python3
"""
Multi-stage training with run continuations.

This demonstrates the run continuation feature - resuming a training run
with config changes while keeping all metrics in a single continuous view.
Continuation points appear as dashed vertical lines on the charts.

Usage:
    python examples/multi_stage_training.py

What you'll see:
    1. Stage 1: Initial training (epochs 0-50) with lr=0.01
    2. Stage 2: Resume with lr=0.001 (epochs 50-100) - first continuation marker
    3. Stage 3: Resume with lr=0.0001 (epochs 100-150) - second continuation marker

The viewer will show vertical dashed lines at epochs 50 and 100 where
the learning rate was changed, similar to deploy markers in Grafana.
"""

import os
import sys
import math
import time
import random

# Add the SDK to the path for development
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "../sdk/python/src"))

# Set the log directory
os.environ["SIXTYSEVEN_LOGDIR"] = "./logs"

from sixtyseven import Run, resume


def simulate_epoch(epoch: int, lr: float, base_loss: float) -> dict:
    """Simulate one training epoch and return metrics."""
    # Loss decreases faster with higher learning rate initially,
    # but needs lower LR to converge to minimum
    lr_factor = math.log10(lr) + 4  # Normalize LR effect
    decay = 0.02 * (1 + lr_factor * 0.5)

    loss = base_loss * math.exp(-decay) + random.gauss(0, 0.02)
    val_loss = loss * 1.15 + random.gauss(0, 0.03)

    accuracy = min(0.99, 1.0 - loss * 0.8 + random.gauss(0, 0.01))
    val_accuracy = min(0.98, accuracy * 0.97 + random.gauss(0, 0.015))

    return {
        "train/loss": max(0.01, loss),
        "train/accuracy": max(0, min(1, accuracy)),
        "val/loss": max(0.01, val_loss),
        "val/accuracy": max(0, min(1, val_accuracy)),
        "learning_rate": lr,
    }


def stage1_initial_training():
    """Stage 1: Initial training with high learning rate."""
    print("=" * 60)
    print("STAGE 1: Initial Training")
    print("=" * 60)
    print("Config: lr=0.01, epochs=0-50")
    print()

    with Run(
        project="multi-stage-demo",
        name=f"staged-training-{int(time.time())}",
        tags=["multi-stage", "demo", "continuation"],
        config={
            "learning_rate": 0.01,
            "batch_size": 64,
            "optimizer": "adam",
            "stage": "initial",
            "model": "resnet18",
        },
        start_server=True,  # Open browser automatically
    ) as run:
        print(f"Run ID: {run.id}")
        print(f"Run directory: {run.logdir}")
        print()

        base_loss = 1.0
        for epoch in range(50):
            metrics = simulate_epoch(epoch, lr=0.01, base_loss=base_loss)
            base_loss = metrics["train/loss"]

            run.log_metrics(metrics, step=epoch)

            if epoch % 10 == 0:
                print(
                    f"  Epoch {epoch:3d}: loss={metrics['train/loss']:.4f}, "
                    f"val_acc={metrics['val/accuracy']:.4f}"
                )

            time.sleep(0.1)  # Simulate training time

        print()
        print(f"Stage 1 complete. Final loss: {base_loss:.4f}")
        print("Loss is plateauing - time to reduce learning rate!")

        # Return run info for continuation
        return run.id, run.logdir, base_loss


def stage2_reduce_lr(run_id: str, run_path: str, base_loss: float):
    """Stage 2: Continue with reduced learning rate."""
    print()
    print("=" * 60)
    print("STAGE 2: Learning Rate Reduction")
    print("=" * 60)
    print("Config: lr=0.001 (10x reduction), epochs=50-100")
    print()

    # Resume the run with new config
    run = resume(
        run_path,  # Path to run directory (local mode)
        config={
            "learning_rate": 0.001,
            "stage": "lr_reduction",
        },
        note="Loss plateaued at ~0.15, reducing LR 10x for better convergence",
    )

    print(f"Resumed run: {run.id}")
    print("A continuation marker will appear at epoch 50 on the chart!")
    print()

    with run:
        for epoch in range(50, 100):
            metrics = simulate_epoch(epoch, lr=0.001, base_loss=base_loss)
            base_loss = metrics["train/loss"]

            run.log_metrics(metrics, step=epoch)

            if epoch % 10 == 0:
                print(
                    f"  Epoch {epoch:3d}: loss={metrics['train/loss']:.4f}, "
                    f"val_acc={metrics['val/accuracy']:.4f}"
                )

            time.sleep(0.1)

        print()
        print(f"Stage 2 complete. Final loss: {base_loss:.4f}")
        print("Getting close to convergence - one more LR reduction!")

        return run_path, base_loss


def stage3_fine_tuning(run_path: str, base_loss: float):
    """Stage 3: Final fine-tuning with very low learning rate."""
    print()
    print("=" * 60)
    print("STAGE 3: Fine-tuning")
    print("=" * 60)
    print("Config: lr=0.0001 (another 10x reduction), epochs=100-150")
    print()

    # Resume again with even lower LR
    run = resume(
        run_path,
        config={
            "learning_rate": 0.0001,
            "stage": "fine_tuning",
            "early_stopping": True,
        },
        note="Final fine-tuning phase with minimal LR",
    )

    print(f"Resumed run: {run.id}")
    print("A second continuation marker will appear at epoch 100!")
    print()

    with run:
        for epoch in range(100, 150):
            metrics = simulate_epoch(epoch, lr=0.0001, base_loss=base_loss)
            base_loss = metrics["train/loss"]

            run.log_metrics(metrics, step=epoch)

            if epoch % 10 == 0:
                print(
                    f"  Epoch {epoch:3d}: loss={metrics['train/loss']:.4f}, "
                    f"val_acc={metrics['val/accuracy']:.4f}"
                )

            time.sleep(0.1)

        print()
        print(f"Stage 3 complete. Final loss: {base_loss:.4f}")


def main():
    print()
    print("Multi-Stage Training with Run Continuations")
    print("=" * 60)
    print()
    print("This demo shows how to use sixtyseven.resume() to continue")
    print("training with config changes. Each continuation creates a")
    print("visual marker on the charts showing where config changed.")
    print()
    print("The viewer server starts once and stays running (like TensorBoard).")
    print("Watch the charts update in real-time across all stages!")
    print()

    # Stage 1: Initial training
    run_id, run_path, loss = stage1_initial_training()

    # Stage 2: Reduce learning rate
    run_path, loss = stage2_reduce_lr(run_id, run_path, loss)

    # Stage 3: Fine-tuning
    stage3_fine_tuning(run_path, loss)

    print()
    print("=" * 60)
    print("Training Complete!")
    print("=" * 60)
    print()
    print("Check the charts in your browser. You should see:")
    print("  - Continuous metrics from epoch 0-150")
    print("  - Dashed vertical line at epoch 50 (first LR reduction)")
    print("  - Dashed vertical line at epoch 100 (second LR reduction)")
    print()
    print("The Config tab will show the final merged config.")
    print("The server is still running - refresh to see final results!")
    print()


if __name__ == "__main__":
    main()
