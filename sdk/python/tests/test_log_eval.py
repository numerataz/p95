"""Tests for the log_eval functionality in p95.Run."""

import json
import os
import tempfile
import time
from unittest import mock


class TestLogEvalLocal:
    """Tests for log_eval in local mode."""

    def test_log_eval_basic(self):
        """Test basic log_eval call in local mode."""
        from p95.run import Run

        with tempfile.TemporaryDirectory() as tmpdir:
            with Run(project="test-project", mode="local", logdir=tmpdir) as run:
                # Log some metrics to advance step
                run.log_metrics({"loss": 0.5}, step=10)

                # Log an eval
                run.log_eval("This output looks great")

            # Check that eval was logged
            run_dir = os.path.join(tmpdir, "test-project", run.name)
            meta_path = os.path.join(run_dir, "meta.json")

            with open(meta_path) as f:
                meta = json.load(f)

            assert "eval_logs" in meta
            assert len(meta["eval_logs"]) == 1
            assert meta["eval_logs"][0]["message"] == "This output looks great"
            assert meta["eval_logs"][0]["step"] == 11  # Step after log_metrics incremented it

    def test_log_eval_with_rating(self):
        """Test log_eval with rating."""
        from p95.run import Run

        with tempfile.TemporaryDirectory() as tmpdir:
            with Run(project="test-project", mode="local", logdir=tmpdir) as run:
                run.log_eval("Bad response", rating="bad")

            run_dir = os.path.join(tmpdir, "test-project", run.name)
            meta_path = os.path.join(run_dir, "meta.json")

            with open(meta_path) as f:
                meta = json.load(f)

            assert meta["eval_logs"][0]["rating"] == "bad"

    def test_log_eval_with_metadata(self):
        """Test log_eval with metadata."""
        from p95.run import Run

        with tempfile.TemporaryDirectory() as tmpdir:
            with Run(project="test-project", mode="local", logdir=tmpdir) as run:
                run.log_eval(
                    "Interesting output",
                    metadata={"sample_id": "abc123", "category": "test"},
                )

            run_dir = os.path.join(tmpdir, "test-project", run.name)
            meta_path = os.path.join(run_dir, "meta.json")

            with open(meta_path) as f:
                meta = json.load(f)

            assert meta["eval_logs"][0]["metadata"]["sample_id"] == "abc123"
            assert meta["eval_logs"][0]["metadata"]["category"] == "test"

    def test_log_eval_multiple(self):
        """Test logging multiple evals."""
        from p95.run import Run

        with tempfile.TemporaryDirectory() as tmpdir:
            with Run(project="test-project", mode="local", logdir=tmpdir) as run:
                run.log_metrics({"loss": 0.5}, step=0)
                run.log_eval("First eval")

                run.log_metrics({"loss": 0.4}, step=1)
                run.log_eval("Second eval")

                run.log_metrics({"loss": 0.3}, step=2)
                run.log_eval("Third eval", rating="good")

            run_dir = os.path.join(tmpdir, "test-project", run.name)
            meta_path = os.path.join(run_dir, "meta.json")

            with open(meta_path) as f:
                meta = json.load(f)

            assert len(meta["eval_logs"]) == 3
            assert meta["eval_logs"][0]["message"] == "First eval"
            assert meta["eval_logs"][1]["message"] == "Second eval"
            assert meta["eval_logs"][2]["message"] == "Third eval"
            assert meta["eval_logs"][2]["rating"] == "good"

    def test_log_eval_uses_current_step(self):
        """Test that log_eval uses the current step value."""
        from p95.run import Run

        with tempfile.TemporaryDirectory() as tmpdir:
            with Run(project="test-project", mode="local", logdir=tmpdir) as run:
                # Log at specific steps
                run.log_metrics({"loss": 0.5}, step=100)
                run.log_eval("At step 100-ish")

                run.log_metrics({"loss": 0.3}, step=200)
                run.log_eval("At step 200-ish")

            run_dir = os.path.join(tmpdir, "test-project", run.name)
            meta_path = os.path.join(run_dir, "meta.json")

            with open(meta_path) as f:
                meta = json.load(f)

            # After log_metrics(step=100), internal step becomes 101
            assert meta["eval_logs"][0]["step"] == 101
            # After log_metrics(step=200), internal step becomes 201
            assert meta["eval_logs"][1]["step"] == 201

    def test_log_eval_has_timestamp(self):
        """Test that log_eval includes a timestamp."""
        from p95.run import Run

        with tempfile.TemporaryDirectory() as tmpdir:
            before = time.time()

            with Run(project="test-project", mode="local", logdir=tmpdir) as run:
                run.log_eval("Timed eval")

            after = time.time()

            run_dir = os.path.join(tmpdir, "test-project", run.name)
            meta_path = os.path.join(run_dir, "meta.json")

            with open(meta_path) as f:
                meta = json.load(f)

            ts = meta["eval_logs"][0]["timestamp"]
            assert before <= ts <= after


class TestLogEvalRemote:
    """Tests for log_eval in remote mode."""

    def test_log_eval_remote_basic(self):
        """Test log_eval in remote mode calls the API."""
        from p95.run import Run

        with mock.patch("p95.client.P95Client") as mock_client_class, \
             mock.patch("p95.metrics.MetricsBatcher") as mock_batcher_class:

            mock_client = mock.MagicMock()
            mock_client.create_run.return_value = "run-123"
            mock_client_class.return_value = mock_client

            mock_batcher = mock.MagicMock()
            mock_batcher_class.return_value = mock_batcher

            # Patch the imports within run module
            with mock.patch.object(Run, "_init_remote_mode"):
                # Create run with mocked internals
                run = object.__new__(Run)
                run._config = mock.MagicMock()
                run._config.mode = "remote"
                run._run_id = "run-123"
                run._remote_client = mock_client
                run._remote_batcher = mock_batcher
                run._step = 0
                run._closed = False
                run._lock = __import__("threading").Lock()

                run.log_eval("Test message", rating="good")

                # Verify log_eval was called on the client
                mock_client.log_eval.assert_called_once()
                call_args = mock_client.log_eval.call_args
                assert call_args[0][0] == "run-123"  # run_id
                assert call_args[0][1]["message"] == "Test message"
                assert call_args[0][1]["rating"] == "good"
                assert "step" in call_args[0][1]
                assert "timestamp" in call_args[0][1]

    def test_log_eval_remote_with_metadata(self):
        """Test log_eval in remote mode with metadata."""
        from p95.run import Run

        mock_client = mock.MagicMock()

        run = object.__new__(Run)
        run._config = mock.MagicMock()
        run._config.mode = "remote"
        run._run_id = "run-456"
        run._remote_client = mock_client
        run._step = 0
        run._closed = False
        run._lock = __import__("threading").Lock()

        run.log_eval(
            "Complex eval",
            rating="neutral",
            metadata={"key": "value", "num": 42},
        )

        call_args = mock_client.log_eval.call_args
        assert call_args[0][1]["metadata"]["key"] == "value"
        assert call_args[0][1]["metadata"]["num"] == 42

    def test_log_eval_remote_no_rating(self):
        """Test log_eval in remote mode without rating."""
        from p95.run import Run

        mock_client = mock.MagicMock()

        run = object.__new__(Run)
        run._config = mock.MagicMock()
        run._config.mode = "remote"
        run._run_id = "run-789"
        run._remote_client = mock_client
        run._step = 5
        run._closed = False
        run._lock = __import__("threading").Lock()

        run.log_eval("Simple message")

        call_args = mock_client.log_eval.call_args
        assert "rating" not in call_args[0][1]
        assert "metadata" not in call_args[0][1]
        assert call_args[0][1]["step"] == 5


class TestLogEvalThreadSafety:
    """Tests for log_eval thread safety."""

    def test_log_eval_thread_safe(self):
        """Test that log_eval is thread-safe."""
        import threading
        from p95.run import Run

        with tempfile.TemporaryDirectory() as tmpdir:
            with Run(project="test-project", mode="local", logdir=tmpdir) as run:
                errors = []

                def log_evals(n):
                    try:
                        for i in range(n):
                            run.log_eval(f"Message from thread at {i}")
                    except Exception as e:
                        errors.append(e)

                threads = [
                    threading.Thread(target=log_evals, args=(10,))
                    for _ in range(5)
                ]

                for t in threads:
                    t.start()
                for t in threads:
                    t.join()

                assert len(errors) == 0

            # Verify all evals were logged
            run_dir = os.path.join(tmpdir, "test-project", run.name)
            meta_path = os.path.join(run_dir, "meta.json")

            with open(meta_path) as f:
                meta = json.load(f)

            # 5 threads * 10 evals each = 50 total
            assert len(meta["eval_logs"]) == 50
