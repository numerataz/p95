#!/usr/bin/env python3
"""Train a model with qualitative evaluation annotations.

This example demonstrates using p95's log_eval() feature to add
human-readable annotations during training. These annotations appear
in the "Notes" tab of the run detail page in the UI.

Usage:
    # Local mode (default)
    python examples/train_with_evals.py

    # Remote mode
    P95_URL=http://localhost:8080 P95_API_KEY=xxx python examples/train_with_evals.py
"""

import os
import time
import numpy as np
import p95


def generate_text_output(epoch: int) -> str:
    """Simulate a model generating text output."""
    outputs = [
        "The quick brown fox jumps over the lazy dog.",
        "Machine learning is transforming how we build software.",
        "The weather today is sunny with a chance of clouds.",
        "Python is a versatile programming language.",
        "Neural networks learn patterns from data.",
        "The cat sat on the mat and looked at the window.",
        "Artificial intelligence is advancing rapidly.",
        "Data science combines statistics and programming.",
        "Deep learning requires large amounts of data.",
        "Transfer learning helps with limited datasets.",
    ]
    # Add some variation based on epoch
    base = outputs[epoch % len(outputs)]
    if epoch > 20:
        # Outputs get more coherent as training progresses
        return base
    elif epoch > 10:
        # Medium quality - some minor issues
        return base.replace("the", "teh").replace(".", "")
    else:
        # Early training - lower quality
        words = base.split()
        np.random.shuffle(words)
        return " ".join(words[:len(words)//2])


def evaluate_output(output: str, epoch: int) -> tuple[str, str]:
    """Simulate human evaluation of model output.

    Returns:
        tuple of (message, rating)
    """
    # Simple heuristics to simulate evaluation
    if len(output) < 20:
        return "Output too short, model not generating enough content", "bad"

    if "teh" in output or not output.endswith("."):
        return "Minor quality issues detected - typos or missing punctuation", "neutral"

    if len(output) > 40 and output[0].isupper():
        return "Good output quality - coherent and well-formed", "good"

    return "Acceptable output, room for improvement", "neutral"


def main():
    config = {
        "epochs": int(os.environ.get("P95_CONFIG_EPOCHS", "30")),
        "lr": float(os.environ.get("P95_CONFIG_LR", "0.001")),
        "batch_size": int(os.environ.get("P95_CONFIG_BATCH_SIZE", "32")),
        "model_type": "transformer",
        "eval_frequency": 5,  # Evaluate every N epochs
    }

    project = os.environ.get("P95_PROJECT", "text-generation")

    print("Training text generation model with qualitative evals")
    print(f"Config: {config}")

    with p95.Run(project=project, config=config) as run:
        print(f"Run ID: {run.id}")
        print(f"Mode: {run.mode}")
        if run.mode == "local":
            print(f"Log dir: {run.logdir}")
            print("\nTo view in UI, run: pnf --logdir <logdir>")
        else:
            print("\nView in UI at: /<team>/<app>/runs/<run_id>")
            print("Look for the 'Notes' tab to see evaluation annotations")

        np.random.seed(42)

        for epoch in range(config["epochs"]):
            # Simulate training metrics
            base_loss = 2.0 * np.exp(-epoch / 10) + 0.1
            loss = base_loss + np.random.normal(0, 0.05)
            perplexity = np.exp(loss)

            run.log_metrics({
                "train/loss": loss,
                "train/perplexity": perplexity,
            }, step=epoch)

            # Periodically evaluate and log qualitative feedback
            if epoch % config["eval_frequency"] == 0:
                # Generate sample output
                output = generate_text_output(epoch)

                # Evaluate the output
                message, rating = evaluate_output(output, epoch)

                # Log the evaluation annotation
                run.log_eval(
                    message=f"Epoch {epoch}: {message}\nSample output: \"{output}\"",
                    rating=rating,
                    metadata={
                        "epoch": epoch,
                        "output_length": len(output),
                        "sample_output": output,
                    }
                )

                print(f"Epoch {epoch}: {rating.upper()} - {message}")

            # Print progress
            if (epoch + 1) % 10 == 0:
                print(f"Epoch {epoch + 1}/{config['epochs']} - loss: {loss:.4f}")

            time.sleep(0.1)  # Simulate training time

        # Final evaluation
        final_output = generate_text_output(config["epochs"])
        run.log_eval(
            message=f"Final model evaluation: Output quality is {'excellent' if len(final_output) > 40 else 'acceptable'}",
            rating="good" if len(final_output) > 40 else "neutral",
            metadata={
                "final_output": final_output,
                "total_epochs": config["epochs"],
            }
        )

        print("\nTraining complete!")
        print(f"Final loss: {loss:.4f}")
        print("\nEvaluation annotations logged. View them in the 'Notes' tab.")


if __name__ == "__main__":
    main()
