#!/usr/bin/env python3
"""
Example demonstrating automatic run sharing with p95.

When share=True, a public share link (https://p95.run/{slug}) is printed
after the run completes. Anyone with the link can view the run without
needing an account.

Usage:
    export P95_URL=https://p.ninetyfive.gg
    export P95_API_KEY=ss67_your_key_here

    python examples/share_run.py
"""

import math
import os
import random
import time

from p95 import Run, configure

configure(
    base_url=os.environ.get("P95_URL", "https://p.ninetyfive.gg"),
)


def main():
    config = {
        "learning_rate": 0.01,
        "batch_size": 64,
        "epochs": 10,
        "optimizer": "sgd",
    }

    with Run(
        project="peepo/peepo",
        name=f"shared-run-{int(time.time())}",
        tags=["example", "shared"],
        config=config,
        share=True,
    ) as run:
        print(f"Run ID: {run.id}")
        print()

        for epoch in range(config["epochs"]):
            loss = math.exp(-0.3 * epoch) + random.gauss(0, 0.02) + 0.05
            accuracy = 1.0 - loss * 0.7 + random.gauss(0, 0.01)

            run.log_metrics(
                {
                    "train/loss": max(0.01, loss),
                    "train/accuracy": max(0.0, min(1.0, accuracy)),
                },
                step=epoch,
            )

            print(f"Epoch {epoch + 1}/{config['epochs']}  loss={loss:.4f}  acc={accuracy:.4f}")
            time.sleep(0.1)


if __name__ == "__main__":
    main()
