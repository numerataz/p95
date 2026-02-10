#!/usr/bin/env python3
"""
Simple MNIST training example with p95 metric tracking.

Usage:
    # First, start the p95 server:
    # cd deployments/docker && docker-compose up -d

    # Install dependencies:
    # pip install torch torchvision
    # pip install -e sdk/python

    # Set your API key:
    # export P95_API_KEY=ss67_your_key_here

    # Run training:
    # python examples/train_mnist.py
"""

import os
import torch
import torch.nn as nn
import torch.optim as optim
from torch.utils.data import DataLoader
from torchvision import datasets, transforms

# Import p95 SDK
from p95 import Run, configure

# Configure p95 (optional - can also use env vars)
configure(
    base_url=os.environ.get("P95_URL", "http://localhost:8080"),
)


class SimpleNet(nn.Module):
    """Simple CNN for MNIST classification."""

    def __init__(self):
        super().__init__()
        self.conv1 = nn.Conv2d(1, 32, 3, 1)
        self.conv2 = nn.Conv2d(32, 64, 3, 1)
        self.dropout1 = nn.Dropout(0.25)
        self.dropout2 = nn.Dropout(0.5)
        self.fc1 = nn.Linear(9216, 128)
        self.fc2 = nn.Linear(128, 10)

    def forward(self, x):
        x = self.conv1(x)
        x = nn.functional.relu(x)
        x = self.conv2(x)
        x = nn.functional.relu(x)
        x = nn.functional.max_pool2d(x, 2)
        x = self.dropout1(x)
        x = torch.flatten(x, 1)
        x = self.fc1(x)
        x = nn.functional.relu(x)
        x = self.dropout2(x)
        x = self.fc2(x)
        return nn.functional.log_softmax(x, dim=1)


def train_epoch(model, device, train_loader, optimizer, epoch, run):
    """Train for one epoch and log metrics to p95."""
    model.train()
    total_loss = 0
    correct = 0
    total = 0

    for batch_idx, (data, target) in enumerate(train_loader):
        data, target = data.to(device), target.to(device)

        optimizer.zero_grad()
        output = model(data)
        loss = nn.functional.nll_loss(output, target)
        loss.backward()
        optimizer.step()

        # Track metrics
        total_loss += loss.item()
        pred = output.argmax(dim=1, keepdim=True)
        correct += pred.eq(target.view_as(pred)).sum().item()
        total += len(data)

        # Log batch metrics every 100 batches
        if batch_idx % 100 == 0:
            step = epoch * len(train_loader) + batch_idx
            run.log_metrics(
                {
                    "train/batch_loss": loss.item(),
                    "train/batch_accuracy": correct / total,
                },
                step=step,
            )

            print(
                f"Epoch {epoch} [{batch_idx * len(data)}/{len(train_loader.dataset)}] "
                f"Loss: {loss.item():.4f}"
            )

    # Log epoch metrics
    avg_loss = total_loss / len(train_loader)
    accuracy = correct / total

    run.log_metrics(
        {
            "train/epoch_loss": avg_loss,
            "train/epoch_accuracy": accuracy,
        },
        step=epoch,
    )

    return avg_loss, accuracy


def evaluate(model, device, test_loader, epoch, run):
    """Evaluate and log validation metrics."""
    model.eval()
    test_loss = 0
    correct = 0

    with torch.no_grad():
        for data, target in test_loader:
            data, target = data.to(device), target.to(device)
            output = model(data)
            test_loss += nn.functional.nll_loss(output, target, reduction="sum").item()
            pred = output.argmax(dim=1, keepdim=True)
            correct += pred.eq(target.view_as(pred)).sum().item()

    test_loss /= len(test_loader.dataset)
    accuracy = correct / len(test_loader.dataset)

    # Log validation metrics
    run.log_metrics(
        {
            "val/loss": test_loss,
            "val/accuracy": accuracy,
        },
        step=epoch,
    )

    print(f"Validation - Loss: {test_loss:.4f}, Accuracy: {accuracy:.4f}")

    return test_loss, accuracy


def main():
    # Hyperparameters
    config = {
        "batch_size": 64,
        "epochs": 5,
        "learning_rate": 0.01,
        "momentum": 0.9,
        "optimizer": "SGD",
        "model": "SimpleNet",
    }

    # Device
    device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
    print(f"Using device: {device}")

    # Data loaders
    transform = transforms.Compose(
        [transforms.ToTensor(), transforms.Normalize((0.1307,), (0.3081,))]
    )

    train_dataset = datasets.MNIST(
        "./data", train=True, download=True, transform=transform
    )
    test_dataset = datasets.MNIST("./data", train=False, transform=transform)

    train_loader = DataLoader(
        train_dataset, batch_size=config["batch_size"], shuffle=True
    )
    test_loader = DataLoader(test_dataset, batch_size=1000, shuffle=False)

    # Model
    model = SimpleNet().to(device)
    optimizer = optim.SGD(
        model.parameters(), lr=config["learning_rate"], momentum=config["momentum"]
    )

    # Start p95 run
    # Replace with your actual team/app slugs
    with Run(
        project="my-team/mnist-experiments",
        name="mnist-simplenet",
        tags=["mnist", "cnn", "pytorch"],
        config=config,
    ) as run:
        print(f"Started run: {run.id}")
        print(f"Project: {run.project}")
        print()

        best_accuracy = 0

        for epoch in range(1, config["epochs"] + 1):
            print(f"\n--- Epoch {epoch}/{config['epochs']} ---")

            # Train
            train_loss, train_acc = train_epoch(
                model, device, train_loader, optimizer, epoch, run
            )

            # Evaluate
            val_loss, val_acc = evaluate(model, device, test_loader, epoch, run)

            # Track best accuracy
            if val_acc > best_accuracy:
                best_accuracy = val_acc
                run.log_metrics({"best_accuracy": best_accuracy}, step=epoch)

            # Log learning rate (could be dynamic with scheduler)
            run.log_metrics({"learning_rate": config["learning_rate"]}, step=epoch)

        print(f"\nTraining complete! Best accuracy: {best_accuracy:.4f}")

        # Log final metrics
        run.log_config({"final_accuracy": best_accuracy})


if __name__ == "__main__":
    main()
