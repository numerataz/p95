"""Tests for p95 evaluation module."""

import json
import os
import tempfile
from unittest import mock

import pytest

from p95.evaluation import (
    Dataset,
    Scorer,
    Evaluation,
    EvaluationConfig,
    EvaluationTarget,
    EvaluationResult,
    EvaluationClient,
)


class TestDataset:
    """Tests for Dataset dataclass."""

    def test_dataset_creation(self):
        """Test creating a dataset with inline data."""
        data = [
            {"input": "What is 2+2?", "expected": "4"},
            {"input": "What is 3+3?", "expected": "6"},
        ]
        dataset = Dataset(name="test-dataset", data=data)

        assert dataset.name == "test-dataset"
        assert dataset.data == data
        assert dataset.format == "json"
        assert dataset.id is None

    def test_dataset_from_list(self):
        """Test creating a dataset from a list."""
        data = [
            {"prompt": "Hello", "answer": "Hi"},
            {"prompt": "Bye", "answer": "Goodbye"},
        ]
        dataset = Dataset.from_list(
            data,
            name="greeting-dataset",
            input_field="prompt",
            expected_field="answer",
        )

        assert dataset.name == "greeting-dataset"
        assert dataset.data == data
        assert dataset.input_field == "prompt"
        assert dataset.expected_field == "answer"
        assert dataset.has_expected is True

    def test_dataset_from_list_no_expected(self):
        """Test creating a dataset without expected field."""
        data = [{"input": "Hello"}, {"input": "World"}]
        dataset = Dataset.from_list(data, name="simple-dataset")

        assert dataset.has_expected is False
        assert dataset.expected_field is None

    def test_dataset_from_file_json(self):
        """Test loading dataset from JSON file."""
        data = [
            {"input": "test1", "expected": "result1"},
            {"input": "test2", "expected": "result2"},
        ]

        with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as f:
            json.dump(data, f)
            f.flush()

            try:
                dataset = Dataset.from_file(f.name)

                assert len(dataset.data) == 2
                assert dataset.format == "json"
                assert dataset.has_expected is True
                assert dataset.expected_field == "expected"
            finally:
                os.unlink(f.name)

    def test_dataset_from_file_jsonl(self):
        """Test loading dataset from JSONL file."""
        lines = [
            '{"input": "line1", "label": "a"}',
            '{"input": "line2", "label": "b"}',
        ]

        with tempfile.NamedTemporaryFile(mode="w", suffix=".jsonl", delete=False) as f:
            f.write("\n".join(lines))
            f.flush()

            try:
                dataset = Dataset.from_file(f.name)

                assert len(dataset.data) == 2
                assert dataset.format == "jsonl"
                assert dataset.has_expected is True
                assert dataset.expected_field == "label"
            finally:
                os.unlink(f.name)

    def test_dataset_from_file_csv(self):
        """Test loading dataset from CSV file."""
        csv_content = "input,target\ntest1,result1\ntest2,result2"

        with tempfile.NamedTemporaryFile(mode="w", suffix=".csv", delete=False) as f:
            f.write(csv_content)
            f.flush()

            try:
                dataset = Dataset.from_file(f.name)

                assert len(dataset.data) == 2
                assert dataset.format == "csv"
                assert dataset.has_expected is True
                assert dataset.expected_field == "target"
            finally:
                os.unlink(f.name)

    def test_dataset_from_file_not_found(self):
        """Test that FileNotFoundError is raised for missing files."""
        with pytest.raises(FileNotFoundError):
            Dataset.from_file("/nonexistent/path/data.json")

    def test_dataset_from_file_unsupported_format(self):
        """Test that ValueError is raised for unsupported formats."""
        with tempfile.NamedTemporaryFile(mode="w", suffix=".txt", delete=False) as f:
            f.write("test content")
            f.flush()

            try:
                with pytest.raises(ValueError) as exc:
                    Dataset.from_file(f.name)
                assert "Unsupported file format" in str(exc.value)
            finally:
                os.unlink(f.name)

    def test_dataset_from_url(self):
        """Test creating a dataset from URL reference."""
        dataset = Dataset.from_url(
            "https://example.com/data.json",
            name="remote-dataset",
            format="json",
        )

        assert dataset.name == "remote-dataset"
        assert dataset.source_url == "https://example.com/data.json"
        assert dataset.data is None
        assert dataset.format == "json"

    def test_dataset_name_from_filename(self):
        """Test that dataset name is inferred from filename."""
        data = [{"input": "test"}]

        with tempfile.NamedTemporaryFile(
            mode="w", suffix=".json", prefix="my_dataset_", delete=False
        ) as f:
            json.dump(data, f)
            f.flush()

            try:
                dataset = Dataset.from_file(f.name)
                # Name should be the stem of the filename
                assert "my_dataset_" in dataset.name
            finally:
                os.unlink(f.name)


class TestScorer:
    """Tests for Scorer dataclass."""

    def test_builtin_scorer_exact_match(self):
        """Test creating an exact_match builtin scorer."""
        scorer = Scorer.builtin("exact_match")

        assert scorer.name == "exact_match"
        assert scorer.type == "builtin"
        assert scorer.config["builtin_name"] == "exact_match"
        assert scorer.requires_expected is True

    def test_builtin_scorer_no_expected(self):
        """Test builtin scorer that doesn't require expected."""
        scorer = Scorer.builtin("length")

        assert scorer.name == "length"
        assert scorer.requires_expected is False

    def test_builtin_scorer_with_params(self):
        """Test builtin scorer with parameters."""
        scorer = Scorer.builtin("bleu", n_gram=4, smooth=True)

        assert scorer.config["parameters"]["n_gram"] == 4
        assert scorer.config["parameters"]["smooth"] is True

    def test_llm_judge_scorer(self):
        """Test creating an LLM-as-judge scorer."""
        scorer = Scorer.llm_judge(
            name="quality-judge",
            model="gpt-4",
            system_prompt="You are a quality judge.",
            user_prompt="Rate this output: {output}",
            output_parser="numeric",
        )

        assert scorer.name == "quality-judge"
        assert scorer.type == "llm_judge"
        assert scorer.config["model"] == "gpt-4"
        assert scorer.config["system_prompt"] == "You are a quality judge."
        assert scorer.config["output_parser"] == "numeric"

    def test_llm_judge_scorer_defaults(self):
        """Test LLM judge with default values."""
        scorer = Scorer.llm_judge(
            name="simple-judge",
            user_prompt="Is this good? {output}",
        )

        assert scorer.config["model"] == "gpt-4o-mini"
        assert scorer.config["system_prompt"] == ""
        assert scorer.config["output_parser"] == "numeric"
        assert scorer.requires_expected is False

    def test_custom_scorer(self):
        """Test creating a custom scorer."""

        def my_scorer(input, output, expected):
            return 1.0 if output == expected else 0.0

        scorer = Scorer.custom("my-scorer", my_scorer, requires_expected=True)

        assert scorer.name == "my-scorer"
        assert scorer.type == "custom"
        assert scorer.config["_local_fn"] == my_scorer
        assert scorer.requires_expected is True


class TestEvaluationTarget:
    """Tests for EvaluationTarget dataclass."""

    def test_target_from_run(self):
        """Test creating target from a run ID."""
        target = EvaluationTarget.from_run("run-abc123", temperature=0.7)

        assert target.run_id == "run-abc123"
        assert target.endpoint is None
        assert target.config["temperature"] == 0.7

    def test_target_from_endpoint(self):
        """Test creating target from an endpoint URL."""
        target = EvaluationTarget.from_endpoint(
            "https://api.openai.com/v1/chat/completions",
            model="gpt-4",
            max_tokens=100,
        )

        assert target.endpoint == "https://api.openai.com/v1/chat/completions"
        assert target.run_id is None
        assert target.config["model"] == "gpt-4"
        assert target.config["max_tokens"] == 100


class TestEvaluationConfig:
    """Tests for EvaluationConfig dataclass."""

    def test_evaluation_config(self):
        """Test creating an evaluation config."""
        dataset = Dataset.from_list([{"input": "test"}], name="test-ds")
        target = EvaluationTarget.from_endpoint("https://example.com/api")
        scorers = [Scorer.builtin("exact_match")]

        config = EvaluationConfig(
            name="test-eval",
            dataset=dataset,
            target=target,
            scorers=scorers,
            description="A test evaluation",
        )

        assert config.name == "test-eval"
        assert config.dataset == dataset
        assert config.target == target
        assert len(config.scorers) == 1
        assert config.description == "A test evaluation"

    def test_evaluation_config_with_ids(self):
        """Test evaluation config with string IDs instead of objects."""
        config = EvaluationConfig(
            name="test-eval",
            dataset="dataset-id-123",
            target=EvaluationTarget.from_endpoint("https://example.com"),
            scorers=["scorer-id-1", "scorer-id-2"],
        )

        assert config.dataset == "dataset-id-123"
        assert config.scorers == ["scorer-id-1", "scorer-id-2"]


class TestEvaluationResult:
    """Tests for EvaluationResult dataclass."""

    def test_evaluation_result(self):
        """Test creating an evaluation result."""
        result = EvaluationResult(
            row_index=0,
            input={"prompt": "Hello"},
            model_output="Hi there!",
            expected="Hello!",
            scores={"exact_match": 0.0, "contains": 1.0},
            scorer_outputs={"exact_match": {"matched": False}},
            latency_ms=150.5,
        )

        assert result.row_index == 0
        assert result.input == {"prompt": "Hello"}
        assert result.model_output == "Hi there!"
        assert result.scores["exact_match"] == 0.0
        assert result.scores["contains"] == 1.0
        assert result.latency_ms == 150.5
        assert result.error is None

    def test_evaluation_result_with_error(self):
        """Test evaluation result with error."""
        result = EvaluationResult(
            row_index=5,
            input={"prompt": "test"},
            model_output=None,
            expected="expected",
            scores={},
            scorer_outputs={},
            error="API timeout",
        )

        assert result.error == "API timeout"
        assert result.model_output is None


class TestEvaluation:
    """Tests for Evaluation dataclass."""

    def test_evaluation_creation(self):
        """Test creating an evaluation."""
        evaluation = Evaluation(
            id="eval-123",
            name="test-evaluation",
            status="pending",
            dataset_id="ds-456",
            scorer_ids=["scorer-1", "scorer-2"],
            target={"endpoint": "https://example.com"},
        )

        assert evaluation.id == "eval-123"
        assert evaluation.name == "test-evaluation"
        assert evaluation.status == "pending"
        assert evaluation.is_complete() is False
        assert evaluation.is_running() is False

    def test_evaluation_is_complete(self):
        """Test is_complete for various statuses."""
        for status in ["completed", "failed", "canceled"]:
            evaluation = Evaluation(
                id="eval-123",
                name="test",
                status=status,
                dataset_id="ds-1",
                scorer_ids=[],
                target={},
            )
            assert evaluation.is_complete() is True

        for status in ["pending", "running"]:
            evaluation = Evaluation(
                id="eval-123",
                name="test",
                status=status,
                dataset_id="ds-1",
                scorer_ids=[],
                target={},
            )
            assert evaluation.is_complete() is False

    def test_evaluation_is_running(self):
        """Test is_running for various statuses."""
        evaluation = Evaluation(
            id="eval-123",
            name="test",
            status="running",
            dataset_id="ds-1",
            scorer_ids=[],
            target={},
        )
        assert evaluation.is_running() is True

        evaluation.status = "pending"
        assert evaluation.is_running() is False

    def test_evaluation_with_scores(self):
        """Test evaluation with overall scores."""
        evaluation = Evaluation(
            id="eval-123",
            name="test",
            status="completed",
            dataset_id="ds-1",
            scorer_ids=["scorer-1"],
            target={},
            overall_scores={"exact_match": 0.85, "bleu": 0.72},
            rows_processed=100,
            rows_failed=5,
        )

        assert evaluation.overall_scores["exact_match"] == 0.85
        assert evaluation.rows_processed == 100
        assert evaluation.rows_failed == 5


class TestEvaluationClient:
    """Tests for EvaluationClient."""

    def test_client_initialization(self):
        """Test client initialization."""
        mock_client = mock.MagicMock()
        eval_client = EvaluationClient(mock_client, "my-team", "my-app")

        assert eval_client.team_slug == "my-team"
        assert eval_client.app_slug == "my-app"
        assert eval_client._base_path == "/teams/my-team/apps/my-app"

    def test_upload_dataset_inline(self):
        """Test uploading inline dataset."""
        mock_client = mock.MagicMock()
        mock_client._request.return_value = {"id": "ds-new-123"}

        eval_client = EvaluationClient(mock_client, "team", "app")
        dataset = Dataset.from_list(
            [{"input": "test", "expected": "result"}],
            name="test-ds",
            expected_field="expected",
        )

        result = eval_client.upload_dataset(dataset)

        assert result == "ds-new-123"
        assert dataset.id == "ds-new-123"
        mock_client._request.assert_called_once()
        call_args = mock_client._request.call_args
        assert call_args[0][0] == "POST"
        assert "/datasets" in call_args[0][1]
        assert call_args[1]["data"]["name"] == "test-ds"
        assert call_args[1]["data"]["source_type"] == "inline"

    def test_upload_dataset_url(self):
        """Test uploading URL-referenced dataset."""
        mock_client = mock.MagicMock()
        mock_client._request.return_value = {"id": "ds-url-456"}

        eval_client = EvaluationClient(mock_client, "team", "app")
        dataset = Dataset.from_url(
            "https://example.com/data.json",
            name="remote-ds",
        )

        result = eval_client.upload_dataset(dataset)

        assert result == "ds-url-456"
        call_args = mock_client._request.call_args
        assert call_args[1]["data"]["source_type"] == "url"
        assert call_args[1]["data"]["source_url"] == "https://example.com/data.json"

    def test_create_scorer(self):
        """Test creating a scorer on the server."""
        mock_client = mock.MagicMock()
        mock_client._request.return_value = {"id": "scorer-new-789"}

        eval_client = EvaluationClient(mock_client, "team", "app")
        scorer = Scorer.builtin("exact_match")

        result = eval_client.create_scorer(scorer)

        assert result == "scorer-new-789"
        assert scorer.id == "scorer-new-789"

    def test_create_scorer_custom_raises(self):
        """Test that creating a custom scorer raises an error."""
        mock_client = mock.MagicMock()
        eval_client = EvaluationClient(mock_client, "team", "app")

        scorer = Scorer.custom("my-scorer", lambda i, o, e: 1.0)

        with pytest.raises(ValueError) as exc:
            eval_client.create_scorer(scorer)
        assert "Custom scorers run locally" in str(exc.value)

    def test_create_evaluation(self):
        """Test creating an evaluation."""
        mock_client = mock.MagicMock()
        mock_client._request.return_value = {
            "id": "eval-new-001",
            "name": "test-eval",
            "status": "pending",
            "dataset_id": "ds-123",
            "scorer_ids": ["scorer-1"],
            "target": {"endpoint": "https://example.com"},
        }

        eval_client = EvaluationClient(mock_client, "team", "app")

        # Create with existing IDs
        config = EvaluationConfig(
            name="test-eval",
            dataset="ds-123",
            target=EvaluationTarget.from_endpoint("https://example.com"),
            scorers=["scorer-1"],
        )

        result = eval_client.create_evaluation(config)

        assert result.id == "eval-new-001"
        assert result.status == "pending"

    def test_get_evaluation(self):
        """Test getting an evaluation by ID."""
        mock_client = mock.MagicMock()
        mock_client._request.return_value = {
            "id": "eval-123",
            "name": "test",
            "status": "completed",
            "dataset_id": "ds-1",
            "scorer_ids": ["s-1"],
            "target": {},
            "overall_scores": {"accuracy": 0.95},
            "rows_processed": 50,
        }

        eval_client = EvaluationClient(mock_client, "team", "app")
        result = eval_client.get("eval-123")

        assert result.id == "eval-123"
        assert result.status == "completed"
        assert result.overall_scores["accuracy"] == 0.95

    def test_get_results(self):
        """Test getting evaluation results."""
        mock_client = mock.MagicMock()
        mock_client._request.return_value = {
            "results": [
                {
                    "row_index": 0,
                    "input": {"text": "hello"},
                    "model_output": "hi",
                    "expected": "hello",
                    "scores": {"exact_match": 0.0},
                    "scorer_outputs": {},
                    "latency_ms": 100.0,
                },
                {
                    "row_index": 1,
                    "input": {"text": "world"},
                    "model_output": "world",
                    "expected": "world",
                    "scores": {"exact_match": 1.0},
                    "scorer_outputs": {},
                    "latency_ms": 95.0,
                },
            ]
        }

        eval_client = EvaluationClient(mock_client, "team", "app")
        results = eval_client.get_results("eval-123")

        assert len(results) == 2
        assert results[0].row_index == 0
        assert results[0].scores["exact_match"] == 0.0
        assert results[1].scores["exact_match"] == 1.0

    def test_cancel_evaluation(self):
        """Test canceling an evaluation."""
        mock_client = mock.MagicMock()
        mock_client._request.return_value = {
            "id": "eval-123",
            "name": "test",
            "status": "canceled",
            "dataset_id": "ds-1",
            "scorer_ids": [],
            "target": {},
        }

        eval_client = EvaluationClient(mock_client, "team", "app")
        result = eval_client.cancel("eval-123")

        assert result.status == "canceled"
        mock_client._request.assert_called_with(
            "POST",
            "/teams/team/apps/app/evaluations/eval-123/cancel",
        )

    def test_list_evaluations(self):
        """Test listing evaluations."""
        mock_client = mock.MagicMock()
        mock_client._request.return_value = {
            "evaluations": [
                {
                    "id": "eval-1",
                    "name": "eval-one",
                    "status": "completed",
                    "dataset_id": "ds-1",
                    "scorer_ids": [],
                    "target": {},
                },
                {
                    "id": "eval-2",
                    "name": "eval-two",
                    "status": "running",
                    "dataset_id": "ds-2",
                    "scorer_ids": [],
                    "target": {},
                },
            ]
        }

        eval_client = EvaluationClient(mock_client, "team", "app")
        results = eval_client.list_evaluations()

        assert len(results) == 2
        assert results[0].name == "eval-one"
        assert results[1].status == "running"

    def test_get_builtin_scorers(self):
        """Test getting builtin scorers."""
        mock_client = mock.MagicMock()
        mock_client._request.return_value = {
            "scorers": [
                {"name": "exact_match", "description": "Exact string match"},
                {"name": "bleu", "description": "BLEU score"},
            ]
        }

        eval_client = EvaluationClient(mock_client, "team", "app")
        result = eval_client.get_builtin_scorers()

        assert len(result) == 2
        assert result[0]["name"] == "exact_match"
