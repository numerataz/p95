# p95 Examples

Example scripts demonstrating how to use the p95 SDK for ML experiment tracking.

## Quick Start

```bash
# Run the TUI viewer
mise run tui

# Run the web viewer
mise run serve

# Run the demo script
mise run demo
```

## Running Examples

### Local Mode Demo

A basic example that simulates training:

```bash
mise run demo
```

Or manually:

```bash
python examples/local_mode_demo.py
```

This example:

- Simulates 500 epochs of training
- Logs train/val loss and accuracy
- Demonstrates all SDK features

### MNIST Training (PyTorch)

A real training example using PyTorch on MNIST:

```bash
pip install torch torchvision
python examples/train_mnist.py
```

## Viewing Results

### TUI (Terminal UI)

```bash
mise run tui
```

Controls:

- `Tab` / `1-4`: Switch panels
- `↑/↓` or `j/k`: Navigate
- `r`: Refresh
- `q`: Quit

### Web UI

```bash
mise run serve
```

Opens http://localhost:6767

## SDK Usage

### Basic Usage

```python
from p95 import Run

with Run(project="my-project", name="experiment-1") as run:
    run.log_config({"lr": 0.001, "batch_size": 32})

    for epoch in range(10):
        loss = train_one_epoch()
        run.log_metrics({"loss": loss}, step=epoch)
```

Runs are saved to `~/.p95/logs` by default.

### Auto-Start Viewer

Use `start_server=True` to automatically launch the web viewer when your training script starts:

```python
from p95 import Run

with Run(
    project="my-project",
    name="experiment-1",
    start_server=True,  # Starts viewer and opens browser
) as run:
    for epoch in range(100):
        run.log_metrics({"loss": loss}, step=epoch)
```

This will:

- Start the p95 server if not already running
- Open your browser directly to the run's metrics page
- Stream metrics in real-time as training progresses
- If the viewer is already open, navigate to the new run automatically

The server stops when the training script exits.
