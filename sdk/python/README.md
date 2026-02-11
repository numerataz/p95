# p95 Python SDK

Track ML experiments locally. No server setup required.

Installing the Python package includes the binary for the TUI and the web server.

## Install

```bash
pip install p95
```

## Usage

```python
from p95 import Run

with Run(project="my-project", name="experiment-1") as run:
    run.log_config({"learning_rate": 0.001, "epochs": 10})

    for epoch in range(10):
        loss = train_one_epoch()
        run.log_metrics({"loss": loss}, step=epoch)
```

## View Results

```bash
# Opens a dashboard at http://localhost:6767
pnf serve --logdir ~/.p95/logs

# Opens the TUI
pnf tui --logdir ~/.p95/logs
```

## API

```python
run.log_metrics({"loss": 0.5, "accuracy": 0.85}, step=epoch)  # Log metrics
run.log_config({"lr": 0.001})                                  # Log config
run.add_tags(["baseline"])                                     # Add tags
```

## Environment Variables

| Variable     | Description        | Default       |
| ------------ | ------------------ | ------------- |
| `P95_LOGDIR` | Where to save logs | `~/.p95/logs` |

## License

MIT
