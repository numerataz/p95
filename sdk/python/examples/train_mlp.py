#!/usr/bin/env python3
"""Train a simple MLP on synthetic classification data.

This example demonstrates using p95 to track a real PyTorch training loop.
It uses a synthetic dataset so no downloads are required.

Usage:
    # Local mode (default)
    python examples/train_mlp.py

    # Remote mode
    P95_URL=http://localhost:8080 P95_API_KEY=xxx python examples/train_mlp.py

    # Or submit as a job
    p95 jobs create --project team/app --command "python train_mlp.py"
"""

import os

# Check if torch is available, use numpy fallback if not
try:
    import torch
    import torch.nn as nn
    import torch.optim as optim

    HAS_TORCH = True
except ImportError:
    HAS_TORCH = False
    print("PyTorch not found, using numpy fallback")

import numpy as np
import p95


def make_synthetic_data(n_samples=1000, n_features=20, n_classes=3, seed=42):
    """Generate synthetic classification data."""
    if HAS_TORCH:
        torch.manual_seed(seed)
        X = torch.randn(n_samples, n_features)
        # Create clusters for each class
        centers = torch.randn(n_classes, n_features) * 2
        y = torch.randint(0, n_classes, (n_samples,))
        for i in range(n_classes):
            mask = y == i
            X[mask] += centers[i]
        return X, y
    else:
        np.random.seed(seed)
        X = np.random.randn(n_samples, n_features).astype(np.float32)
        centers = np.random.randn(n_classes, n_features).astype(np.float32) * 2
        y = np.random.randint(0, n_classes, n_samples)
        for i in range(n_classes):
            mask = y == i
            X[mask] += centers[i]
        return X, y


if HAS_TORCH:

    class MLP(nn.Module):
        """Simple MLP classifier."""

        def __init__(self, n_features, n_hidden, n_classes, dropout=0.2):
            super().__init__()
            self.net = nn.Sequential(
                nn.Linear(n_features, n_hidden),
                nn.ReLU(),
                nn.Dropout(dropout),
                nn.Linear(n_hidden, n_hidden),
                nn.ReLU(),
                nn.Dropout(dropout),
                nn.Linear(n_hidden, n_classes),
            )

        def forward(self, x):
            return self.net(x)


class NumpyMLP:
    """Simple numpy-based MLP for when PyTorch isn't available."""

    def __init__(self, n_features, n_hidden, n_classes, lr=0.01):
        self.lr = lr
        # Xavier initialization
        self.W1 = np.random.randn(n_features, n_hidden).astype(np.float32) * np.sqrt(
            2.0 / n_features
        )
        self.b1 = np.zeros(n_hidden, dtype=np.float32)
        self.W2 = np.random.randn(n_hidden, n_hidden).astype(np.float32) * np.sqrt(
            2.0 / n_hidden
        )
        self.b2 = np.zeros(n_hidden, dtype=np.float32)
        self.W3 = np.random.randn(n_hidden, n_classes).astype(np.float32) * np.sqrt(
            2.0 / n_hidden
        )
        self.b3 = np.zeros(n_classes, dtype=np.float32)

    def relu(self, x):
        return np.maximum(0, x)

    def softmax(self, x):
        exp_x = np.exp(x - np.max(x, axis=1, keepdims=True))
        return exp_x / np.sum(exp_x, axis=1, keepdims=True)

    def forward(self, X):
        self.z1 = X @ self.W1 + self.b1
        self.a1 = self.relu(self.z1)
        self.z2 = self.a1 @ self.W2 + self.b2
        self.a2 = self.relu(self.z2)
        self.z3 = self.a2 @ self.W3 + self.b3
        return self.softmax(self.z3)

    def loss(self, probs, y):
        n = len(y)
        log_probs = -np.log(probs[np.arange(n), y] + 1e-8)
        return np.mean(log_probs)

    def accuracy(self, probs, y):
        preds = np.argmax(probs, axis=1)
        return np.mean(preds == y)

    def backward(self, X, y, probs):
        n = len(y)
        # Gradient of cross-entropy loss
        dz3 = probs.copy()
        dz3[np.arange(n), y] -= 1
        dz3 /= n

        dW3 = self.a2.T @ dz3
        db3 = np.sum(dz3, axis=0)

        da2 = dz3 @ self.W3.T
        dz2 = da2 * (self.z2 > 0)

        dW2 = self.a1.T @ dz2
        db2 = np.sum(dz2, axis=0)

        da1 = dz2 @ self.W2.T
        dz1 = da1 * (self.z1 > 0)

        dW1 = X.T @ dz1
        db1 = np.sum(dz1, axis=0)

        # Update weights
        self.W3 -= self.lr * dW3
        self.b3 -= self.lr * db3
        self.W2 -= self.lr * dW2
        self.b2 -= self.lr * db2
        self.W1 -= self.lr * dW1
        self.b1 -= self.lr * db1


def train_torch(run, config):
    """Train using PyTorch."""
    # Generate data
    X_train, y_train = make_synthetic_data(
        n_samples=config["n_samples"],
        n_features=config["n_features"],
        n_classes=config["n_classes"],
    )
    X_val, y_val = make_synthetic_data(
        n_samples=config["n_samples"] // 5,
        n_features=config["n_features"],
        n_classes=config["n_classes"],
        seed=123,
    )

    # Create model
    model = MLP(
        n_features=config["n_features"],
        n_hidden=config["n_hidden"],
        n_classes=config["n_classes"],
        dropout=config["dropout"],
    )

    criterion = nn.CrossEntropyLoss()
    optimizer = optim.Adam(model.parameters(), lr=config["lr"])

    batch_size = config["batch_size"]
    n_batches = len(X_train) // batch_size

    for epoch in range(config["epochs"]):
        model.train()
        epoch_loss = 0.0
        correct = 0
        total = 0

        # Shuffle data
        perm = torch.randperm(len(X_train))
        X_train = X_train[perm]
        y_train = y_train[perm]

        for batch_idx in range(n_batches):
            start = batch_idx * batch_size
            end = start + batch_size
            X_batch = X_train[start:end]
            y_batch = y_train[start:end]

            optimizer.zero_grad()
            outputs = model(X_batch)
            loss = criterion(outputs, y_batch)
            loss.backward()
            optimizer.step()

            epoch_loss += loss.item()
            _, predicted = outputs.max(1)
            total += y_batch.size(0)
            correct += predicted.eq(y_batch).sum().item()

        train_loss = epoch_loss / n_batches
        train_acc = correct / total

        # Validation
        model.eval()
        with torch.no_grad():
            val_outputs = model(X_val)
            val_loss = criterion(val_outputs, y_val).item()
            _, val_predicted = val_outputs.max(1)
            val_acc = val_predicted.eq(y_val).sum().item() / len(y_val)

        # Log metrics
        run.log_metrics(
            {
                "train/loss": train_loss,
                "train/accuracy": train_acc,
                "val/loss": val_loss,
                "val/accuracy": val_acc,
            },
            step=epoch,
        )

        # Check for interventions
        intervention = run.check_intervention()
        if intervention:
            print(f"Received intervention: {intervention['type']}")
            run.apply_intervention(intervention)
            # Update config if adjusted
            if intervention["type"] == "adjust_config":
                for key, value in intervention.get("config_delta", {}).items():
                    if key == "lr":
                        for param_group in optimizer.param_groups:
                            param_group["lr"] = value
                        print(f"Updated learning rate to {value}")

        print(
            f"Epoch {epoch + 1}/{config['epochs']} - "
            f"train_loss: {train_loss:.4f}, train_acc: {train_acc:.4f}, "
            f"val_loss: {val_loss:.4f}, val_acc: {val_acc:.4f}"
        )


def train_numpy(run, config):
    """Train using numpy fallback."""
    # Generate data
    X_train, y_train = make_synthetic_data(
        n_samples=config["n_samples"],
        n_features=config["n_features"],
        n_classes=config["n_classes"],
    )
    X_val, y_val = make_synthetic_data(
        n_samples=config["n_samples"] // 5,
        n_features=config["n_features"],
        n_classes=config["n_classes"],
        seed=123,
    )

    model = NumpyMLP(
        n_features=config["n_features"],
        n_hidden=config["n_hidden"],
        n_classes=config["n_classes"],
        lr=config["lr"],
    )

    batch_size = config["batch_size"]
    n_batches = len(X_train) // batch_size

    for epoch in range(config["epochs"]):
        epoch_loss = 0.0
        epoch_acc = 0.0

        # Shuffle data
        perm = np.random.permutation(len(X_train))
        X_train = X_train[perm]
        y_train = y_train[perm]

        for batch_idx in range(n_batches):
            start = batch_idx * batch_size
            end = start + batch_size
            X_batch = X_train[start:end]
            y_batch = y_train[start:end]

            probs = model.forward(X_batch)
            loss = model.loss(probs, y_batch)
            acc = model.accuracy(probs, y_batch)
            model.backward(X_batch, y_batch, probs)

            epoch_loss += loss
            epoch_acc += acc

        train_loss = epoch_loss / n_batches
        train_acc = epoch_acc / n_batches

        # Validation
        val_probs = model.forward(X_val)
        val_loss = model.loss(val_probs, y_val)
        val_acc = model.accuracy(val_probs, y_val)

        # Log metrics
        run.log_metrics(
            {
                "train/loss": train_loss,
                "train/accuracy": train_acc,
                "val/loss": val_loss,
                "val/accuracy": val_acc,
            },
            step=epoch,
        )

        # Check for interventions
        intervention = run.check_intervention()
        if intervention:
            print(f"Received intervention: {intervention['type']}")
            run.apply_intervention(intervention)
            if intervention["type"] == "adjust_config":
                for key, value in intervention.get("config_delta", {}).items():
                    if key == "lr":
                        model.lr = value
                        print(f"Updated learning rate to {value}")

        print(
            f"Epoch {epoch + 1}/{config['epochs']} - "
            f"train_loss: {train_loss:.4f}, train_acc: {train_acc:.4f}, "
            f"val_loss: {val_loss:.4f}, val_acc: {val_acc:.4f}"
        )


def main():
    # Get config from environment or use defaults
    config = {
        "epochs": int(os.environ.get("P95_CONFIG_EPOCHS", "50")),
        "lr": float(os.environ.get("P95_CONFIG_LR", "0.001")),
        "batch_size": int(os.environ.get("P95_CONFIG_BATCH_SIZE", "32")),
        "n_hidden": int(os.environ.get("P95_CONFIG_N_HIDDEN", "64")),
        "n_samples": int(os.environ.get("P95_CONFIG_N_SAMPLES", "1000")),
        "n_features": int(os.environ.get("P95_CONFIG_N_FEATURES", "20")),
        "n_classes": int(os.environ.get("P95_CONFIG_N_CLASSES", "3")),
        "dropout": float(os.environ.get("P95_CONFIG_DROPOUT", "0.2")),
    }

    # Determine project - use env var if set (for remote mode)
    project = os.environ.get("P95_PROJECT", "mlp-training")

    print("Training MLP classifier")
    print(f"Config: {config}")
    print(f"Backend: {'PyTorch' if HAS_TORCH else 'NumPy'}")

    with p95.Run(project=project, config=config) as run:
        print(f"Run ID: {run.id}")

        if HAS_TORCH:
            train_torch(run, config)
        else:
            train_numpy(run, config)

        print("Training complete!")


if __name__ == "__main__":
    main()
