# Sixtyseven Python SDK

Track ML experiments locally. No server setup required.

## Install

```bash
pip install sixtyseven
```

## Usage

```python
from sixtyseven import Run

with Run(project="my-project", name="experiment-1") as run:
    run.log_config({"learning_rate": 0.001, "epochs": 10})

    for epoch in range(10):
        loss = train_one_epoch()
        run.log_metrics({"loss": loss}, step=epoch)
```

## View Results

```bash
sixtyseven --logdir ~/.sixtyseven/logs
```

Opens a dashboard at http://localhost:6767

## API

```python
run.log_metrics({"loss": 0.5, "accuracy": 0.85}, step=epoch)  # Log metrics
run.log_config({"lr": 0.001})                                  # Log config
run.add_tags(["baseline"])                                     # Add tags
```

## Environment Variables

| Variable            | Description        | Default              |
| ------------------- | ------------------ | -------------------- |
| `SIXTYSEVEN_LOGDIR` | Where to save logs | `~/.sixtyseven/logs` |

## License

MIT
