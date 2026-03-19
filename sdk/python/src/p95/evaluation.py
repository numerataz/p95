"""Evaluation module for p95 SDK.

This module provides functionality for running evaluations against
models or endpoints using datasets and scorers.
"""

import json
import time
from dataclasses import dataclass, field
from typing import Any, Callable, Dict, List, Optional, Union
from pathlib import Path

from p95.client import P95Client
from p95.config import SDKConfig


@dataclass
class Dataset:
    """Represents an evaluation dataset.

    Can be created from:
    - A local file (JSON, JSONL, CSV)
    - A pandas DataFrame
    - An external URL
    - Inline data (list of dicts)
    """
    name: str
    data: Optional[List[Dict[str, Any]]] = None
    source_url: Optional[str] = None
    format: str = "json"
    has_expected: bool = False
    input_field: str = "input"
    expected_field: Optional[str] = None

    # Set after upload
    id: Optional[str] = None

    @classmethod
    def from_file(cls, path: str, name: Optional[str] = None) -> "Dataset":
        """Create a dataset from a local file.

        Args:
            path: Path to the file (JSON, JSONL, or CSV)
            name: Optional name for the dataset

        Returns:
            Dataset instance with loaded data
        """
        filepath = Path(path)
        if not filepath.exists():
            raise FileNotFoundError(f"File not found: {path}")

        name = name or filepath.stem

        # Determine format from extension
        ext = filepath.suffix.lower()
        if ext == ".json":
            with open(filepath) as f:
                data = json.load(f)
            if not isinstance(data, list):
                data = [data]
            fmt = "json"
        elif ext == ".jsonl":
            data = []
            with open(filepath) as f:
                for line in f:
                    if line.strip():
                        data.append(json.loads(line))
            fmt = "jsonl"
        elif ext == ".csv":
            import csv
            data = []
            with open(filepath) as f:
                reader = csv.DictReader(f)
                for row in reader:
                    data.append(row)
            fmt = "csv"
        else:
            raise ValueError(f"Unsupported file format: {ext}")

        # Detect if dataset has expected field
        has_expected = False
        expected_field = None
        if data:
            sample = data[0]
            for field_name in ["expected", "ground_truth", "answer", "label", "target"]:
                if field_name in sample:
                    has_expected = True
                    expected_field = field_name
                    break

        return cls(
            name=name,
            data=data,
            format=fmt,
            has_expected=has_expected,
            expected_field=expected_field,
        )

    @classmethod
    def from_dataframe(cls, df: "pandas.DataFrame", name: str) -> "Dataset":
        """Create a dataset from a pandas DataFrame.

        Args:
            df: pandas DataFrame
            name: Name for the dataset

        Returns:
            Dataset instance with DataFrame data
        """
        data = df.to_dict(orient="records")

        # Detect expected field
        has_expected = False
        expected_field = None
        for field_name in ["expected", "ground_truth", "answer", "label", "target"]:
            if field_name in df.columns:
                has_expected = True
                expected_field = field_name
                break

        return cls(
            name=name,
            data=data,
            format="json",
            has_expected=has_expected,
            expected_field=expected_field,
        )

    @classmethod
    def from_url(cls, url: str, name: str, format: str = "json") -> "Dataset":
        """Create a dataset from an external URL.

        Args:
            url: URL to the dataset
            name: Name for the dataset
            format: Data format (json, jsonl, csv)

        Returns:
            Dataset instance referencing the URL
        """
        return cls(
            name=name,
            source_url=url,
            format=format,
        )

    @classmethod
    def from_list(
        cls,
        data: List[Dict[str, Any]],
        name: str,
        input_field: str = "input",
        expected_field: Optional[str] = None,
    ) -> "Dataset":
        """Create a dataset from a list of dictionaries.

        Args:
            data: List of dictionaries with input/output pairs
            name: Name for the dataset
            input_field: Field name for input data
            expected_field: Field name for expected output

        Returns:
            Dataset instance
        """
        return cls(
            name=name,
            data=data,
            format="json",
            has_expected=expected_field is not None,
            input_field=input_field,
            expected_field=expected_field,
        )


@dataclass
class Scorer:
    """Represents a scorer for evaluating model outputs.

    Scorers can be:
    - Builtin (accuracy, bleu, rouge, etc.)
    - LLM-as-judge (uses an LLM to evaluate)
    - Custom (user-defined Python function)
    """
    name: str
    type: str  # "builtin", "llm_judge", "custom"
    config: Dict[str, Any] = field(default_factory=dict)
    requires_expected: bool = False

    # Set after creation on server
    id: Optional[str] = None

    @classmethod
    def builtin(cls, name: str, **params) -> "Scorer":
        """Create a builtin scorer.

        Available builtin scorers:
        - exact_match: Exact string match
        - contains: Substring match
        - bleu: BLEU score for text generation
        - rouge-1, rouge-2, rouge-l: ROUGE scores
        - accuracy: Classification accuracy
        - f1, precision, recall: Classification metrics
        - length: Response character length
        - word_count: Response word count
        - json_valid: Check if output is valid JSON
        - toxicity: Basic toxicity detection

        Args:
            name: Name of the builtin scorer
            **params: Additional parameters for the scorer

        Returns:
            Scorer instance
        """
        # Scorers that require expected output
        requires_expected = name in [
            "exact_match", "contains", "bleu",
            "rouge-1", "rouge-2", "rouge-l",
            "accuracy", "f1", "precision", "recall",
        ]

        return cls(
            name=name,
            type="builtin",
            config={
                "builtin_name": name,
                "parameters": params,
            },
            requires_expected=requires_expected,
        )

    @classmethod
    def llm_judge(
        cls,
        name: str,
        model: str = "gpt-4o-mini",
        system_prompt: str = "",
        user_prompt: str = "",
        output_parser: str = "numeric",
        requires_expected: bool = False,
    ) -> "Scorer":
        """Create an LLM-as-judge scorer.

        The user_prompt can contain template variables:
        - {input}: The input from the dataset
        - {output}: The model's output
        - {expected}: The expected output (if available)

        Args:
            name: Name for this scorer
            model: LLM model to use (e.g., "gpt-4", "claude-3-opus")
            system_prompt: System prompt for the judge
            user_prompt: User prompt template
            output_parser: How to parse the response ("numeric", "boolean", "json")
            requires_expected: Whether this scorer needs ground truth

        Returns:
            Scorer instance
        """
        return cls(
            name=name,
            type="llm_judge",
            config={
                "model": model,
                "system_prompt": system_prompt,
                "user_prompt": user_prompt,
                "output_parser": output_parser,
            },
            requires_expected=requires_expected,
        )

    @classmethod
    def custom(
        cls,
        name: str,
        fn: Callable[[Any, Any, Any], float],
        requires_expected: bool = False,
    ) -> "Scorer":
        """Create a custom scorer from a Python function.

        The function signature should be:
            fn(input, output, expected) -> float

        Note: Custom scorers run locally, not on the server.

        Args:
            name: Name for this scorer
            fn: Scoring function
            requires_expected: Whether this scorer needs ground truth

        Returns:
            Scorer instance
        """
        return cls(
            name=name,
            type="custom",
            config={
                "_local_fn": fn,
            },
            requires_expected=requires_expected,
        )


@dataclass
class EvaluationTarget:
    """Specifies what to evaluate."""
    run_id: Optional[str] = None
    endpoint: Optional[str] = None
    config: Dict[str, Any] = field(default_factory=dict)

    @classmethod
    def from_run(cls, run_id: str, **config) -> "EvaluationTarget":
        """Evaluate a trained model from a run."""
        return cls(run_id=run_id, config=config)

    @classmethod
    def from_endpoint(cls, url: str, **config) -> "EvaluationTarget":
        """Evaluate an external API endpoint."""
        return cls(endpoint=url, config=config)


@dataclass
class EvaluationConfig:
    """Configuration for an evaluation."""
    name: str
    dataset: Union[Dataset, str]  # Dataset object or ID
    target: EvaluationTarget
    scorers: List[Union[Scorer, str]] = field(default_factory=list)  # Scorer objects or IDs
    description: Optional[str] = None
    config: Dict[str, Any] = field(default_factory=dict)


@dataclass
class EvaluationResult:
    """Result of a single evaluation row."""
    row_index: int
    input: Any
    model_output: Any
    expected: Any
    scores: Dict[str, float]
    scorer_outputs: Dict[str, Any]
    latency_ms: Optional[float] = None
    error: Optional[str] = None


@dataclass
class Evaluation:
    """Represents an evaluation run."""
    id: str
    name: str
    status: str
    dataset_id: str
    scorer_ids: List[str]
    target: Dict[str, Any]
    overall_scores: Optional[Dict[str, float]] = None
    rows_processed: int = 0
    rows_failed: int = 0
    created_at: Optional[str] = None

    def is_complete(self) -> bool:
        return self.status in ["completed", "failed", "canceled"]

    def is_running(self) -> bool:
        return self.status == "running"


class EvaluationClient:
    """Client for managing evaluations."""

    def __init__(self, client: P95Client, team_slug: str, app_slug: str):
        """
        Initialize the evaluation client.

        Args:
            client: P95 API client
            team_slug: Team slug
            app_slug: App slug
        """
        self.client = client
        self.team_slug = team_slug
        self.app_slug = app_slug
        self._base_path = f"/teams/{team_slug}/apps/{app_slug}"

    def upload_dataset(self, dataset: Dataset) -> str:
        """
        Upload a dataset to the server.

        Args:
            dataset: Dataset to upload

        Returns:
            Dataset ID
        """
        data = {
            "name": dataset.name,
            "format": dataset.format,
            "has_expected": dataset.has_expected,
            "input_field": dataset.input_field,
        }

        if dataset.expected_field:
            data["expected_field"] = dataset.expected_field

        if dataset.source_url:
            data["source_type"] = "url"
            data["source_url"] = dataset.source_url
        else:
            data["source_type"] = "inline"
            data["data"] = dataset.data

        response = self.client._request(
            "POST",
            f"{self._base_path}/datasets",
            data=data,
        )

        dataset.id = response["id"]
        return response["id"]

    def create_scorer(self, scorer: Scorer) -> str:
        """
        Create a scorer on the server.

        Args:
            scorer: Scorer to create

        Returns:
            Scorer ID
        """
        if scorer.type == "custom":
            raise ValueError("Custom scorers run locally and cannot be uploaded")

        data = {
            "name": scorer.name,
            "type": scorer.type,
            "config": scorer.config,
            "requires_expected": scorer.requires_expected,
        }

        response = self.client._request(
            "POST",
            f"{self._base_path}/scorers",
            data=data,
        )

        scorer.id = response["id"]
        return response["id"]

    def create_evaluation(self, config: EvaluationConfig, start: bool = False) -> Evaluation:
        """
        Create an evaluation.

        Args:
            config: Evaluation configuration
            start: Whether to start the evaluation immediately

        Returns:
            Evaluation instance
        """
        # Upload dataset if needed
        dataset_id = config.dataset if isinstance(config.dataset, str) else config.dataset.id
        if not dataset_id:
            if isinstance(config.dataset, Dataset):
                dataset_id = self.upload_dataset(config.dataset)
            else:
                raise ValueError("Dataset must be uploaded or provide an ID")

        # Create scorers if needed
        scorer_ids = []
        custom_scorers = []
        for scorer in config.scorers:
            if isinstance(scorer, str):
                scorer_ids.append(scorer)
            elif scorer.type == "custom":
                custom_scorers.append(scorer)
            elif not scorer.id:
                scorer_id = self.create_scorer(scorer)
                scorer_ids.append(scorer_id)
            else:
                scorer_ids.append(scorer.id)

        # Build target
        target = {}
        if config.target.run_id:
            target["run_id"] = config.target.run_id
        if config.target.endpoint:
            target["endpoint"] = config.target.endpoint
        if config.target.config:
            target["config"] = config.target.config

        data = {
            "name": config.name,
            "description": config.description,
            "dataset_id": dataset_id,
            "target": target,
            "scorer_ids": scorer_ids,
            "config": config.config,
        }

        response = self.client._request(
            "POST",
            f"{self._base_path}/evaluations",
            data=data,
        )

        evaluation = Evaluation(
            id=response["id"],
            name=response["name"],
            status=response["status"],
            dataset_id=response["dataset_id"],
            scorer_ids=response["scorer_ids"],
            target=response["target"],
            created_at=response.get("created_at"),
        )

        if start:
            return self.start(evaluation.id)

        return evaluation

    def start(self, evaluation_id: str) -> Evaluation:
        """
        Start an evaluation.

        Args:
            evaluation_id: Evaluation ID

        Returns:
            Updated Evaluation instance
        """
        response = self.client._request(
            "POST",
            f"{self._base_path}/evaluations/{evaluation_id}/start",
        )

        return Evaluation(
            id=response["id"],
            name=response["name"],
            status=response["status"],
            dataset_id=response["dataset_id"],
            scorer_ids=response["scorer_ids"],
            target=response["target"],
            overall_scores=response.get("overall_scores"),
            rows_processed=response.get("rows_processed", 0),
            rows_failed=response.get("rows_failed", 0),
            created_at=response.get("created_at"),
        )

    def get(self, evaluation_id: str) -> Evaluation:
        """
        Get an evaluation by ID.

        Args:
            evaluation_id: Evaluation ID

        Returns:
            Evaluation instance
        """
        response = self.client._request(
            "GET",
            f"{self._base_path}/evaluations/{evaluation_id}",
        )

        return Evaluation(
            id=response["id"],
            name=response["name"],
            status=response["status"],
            dataset_id=response["dataset_id"],
            scorer_ids=response["scorer_ids"],
            target=response["target"],
            overall_scores=response.get("overall_scores"),
            rows_processed=response.get("rows_processed", 0),
            rows_failed=response.get("rows_failed", 0),
            created_at=response.get("created_at"),
        )

    def wait(self, evaluation_id: str, poll_interval: float = 2.0, timeout: Optional[float] = None) -> Evaluation:
        """
        Wait for an evaluation to complete.

        Args:
            evaluation_id: Evaluation ID
            poll_interval: Seconds between status checks
            timeout: Maximum seconds to wait (None = no timeout)

        Returns:
            Completed Evaluation instance
        """
        start_time = time.time()

        while True:
            evaluation = self.get(evaluation_id)

            if evaluation.is_complete():
                return evaluation

            if timeout and (time.time() - start_time) > timeout:
                raise TimeoutError(f"Evaluation {evaluation_id} did not complete within {timeout} seconds")

            time.sleep(poll_interval)

    def get_results(self, evaluation_id: str, limit: int = 100, offset: int = 0) -> List[EvaluationResult]:
        """
        Get results for an evaluation.

        Args:
            evaluation_id: Evaluation ID
            limit: Maximum results to return
            offset: Offset for pagination

        Returns:
            List of EvaluationResult instances
        """
        response = self.client._request(
            "GET",
            f"{self._base_path}/evaluations/{evaluation_id}/results",
            params={"limit": limit, "offset": offset},
        )

        results = []
        for item in response.get("results", []):
            results.append(EvaluationResult(
                row_index=item["row_index"],
                input=item["input"],
                model_output=item.get("model_output"),
                expected=item.get("expected"),
                scores=item.get("scores", {}),
                scorer_outputs=item.get("scorer_outputs", {}),
                latency_ms=item.get("latency_ms"),
                error=item.get("error"),
            ))

        return results

    def get_scores_summary(self, evaluation_id: str) -> Dict[str, Dict[str, float]]:
        """
        Get aggregated scores for an evaluation.

        Args:
            evaluation_id: Evaluation ID

        Returns:
            Dictionary mapping scorer names to summary stats
        """
        response = self.client._request(
            "GET",
            f"{self._base_path}/evaluations/{evaluation_id}/scores",
        )

        return response.get("scorer_summaries", {})

    def cancel(self, evaluation_id: str) -> Evaluation:
        """
        Cancel a running evaluation.

        Args:
            evaluation_id: Evaluation ID

        Returns:
            Updated Evaluation instance
        """
        response = self.client._request(
            "POST",
            f"{self._base_path}/evaluations/{evaluation_id}/cancel",
        )

        return Evaluation(
            id=response["id"],
            name=response["name"],
            status=response["status"],
            dataset_id=response["dataset_id"],
            scorer_ids=response["scorer_ids"],
            target=response["target"],
        )

    def list_datasets(self, limit: int = 50, offset: int = 0) -> List[Dict[str, Any]]:
        """List datasets in the app."""
        response = self.client._request(
            "GET",
            f"{self._base_path}/datasets",
            params={"limit": limit, "offset": offset},
        )
        return response.get("datasets", [])

    def list_scorers(self, limit: int = 50, offset: int = 0) -> List[Dict[str, Any]]:
        """List scorers in the app."""
        response = self.client._request(
            "GET",
            f"{self._base_path}/scorers",
            params={"limit": limit, "offset": offset},
        )
        return response.get("scorers", [])

    def list_evaluations(self, limit: int = 50, offset: int = 0, status: Optional[str] = None) -> List[Evaluation]:
        """List evaluations in the app."""
        params = {"limit": limit, "offset": offset}
        if status:
            params["status"] = status

        response = self.client._request(
            "GET",
            f"{self._base_path}/evaluations",
            params=params,
        )

        evaluations = []
        for item in response.get("evaluations", []):
            evaluations.append(Evaluation(
                id=item["id"],
                name=item["name"],
                status=item["status"],
                dataset_id=item["dataset_id"],
                scorer_ids=item["scorer_ids"],
                target=item["target"],
                overall_scores=item.get("overall_scores"),
                rows_processed=item.get("rows_processed", 0),
                rows_failed=item.get("rows_failed", 0),
                created_at=item.get("created_at"),
            ))

        return evaluations

    def get_builtin_scorers(self) -> List[Dict[str, Any]]:
        """Get list of available builtin scorers."""
        response = self.client._request(
            "GET",
            f"{self._base_path}/scorers/builtin",
        )
        return response.get("scorers", [])


# Convenience functions for quick evaluations


def evaluate(
    project: str,
    dataset: Union[Dataset, str, List[Dict[str, Any]]],
    target: Union[EvaluationTarget, str],
    scorers: List[Union[Scorer, str]],
    name: Optional[str] = None,
    wait: bool = True,
    api_key: Optional[str] = None,
) -> Evaluation:
    """
    Run an evaluation.

    This is a convenience function for quick evaluations.

    Args:
        project: Project in format "team/app"
        dataset: Dataset object, ID, or list of dicts
        target: EvaluationTarget or endpoint URL
        scorers: List of Scorer objects or builtin scorer names
        name: Optional evaluation name
        wait: Whether to wait for completion
        api_key: Optional API key

    Returns:
        Completed Evaluation instance

    Example:
        result = p95.evaluate(
            project="my-team/my-app",
            dataset=[
                {"input": "What is 2+2?", "expected": "4"},
                {"input": "What is 3+3?", "expected": "6"},
            ],
            target="https://api.openai.com/v1/chat/completions",
            scorers=["exact_match", "contains"],
        )
        print(result.overall_scores)
    """
    # Parse project
    parts = project.split("/")
    if len(parts) != 2:
        raise ValueError("Project must be in format 'team/app'")
    team_slug, app_slug = parts

    # Create client
    config = SDKConfig.from_env()
    if api_key:
        config.api_key = api_key
    client = P95Client(config)
    eval_client = EvaluationClient(client, team_slug, app_slug)

    # Convert dataset if needed
    if isinstance(dataset, list):
        dataset = Dataset.from_list(dataset, name or "inline-dataset")

    # Convert target if needed
    if isinstance(target, str):
        target = EvaluationTarget.from_endpoint(target)

    # Convert scorer names to Scorer objects
    processed_scorers = []
    for scorer in scorers:
        if isinstance(scorer, str):
            processed_scorers.append(Scorer.builtin(scorer))
        else:
            processed_scorers.append(scorer)

    # Create evaluation config
    eval_config = EvaluationConfig(
        name=name or f"evaluation-{int(time.time())}",
        dataset=dataset,
        target=target,
        scorers=processed_scorers,
    )

    # Run evaluation
    evaluation = eval_client.create_evaluation(eval_config, start=True)

    if wait:
        return eval_client.wait(evaluation.id)

    return evaluation
