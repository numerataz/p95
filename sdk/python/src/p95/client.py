"""HTTP client for communicating with the p95 API."""

import time
from typing import Any, Dict, List, Optional
from urllib.parse import urljoin

import requests

from p95.config import SDKConfig
from p95.exceptions import APIError, AuthenticationError


class P95Client:
    """HTTP client for the p95 API."""

    def __init__(self, config: SDKConfig, api_key: Optional[str] = None):
        """
        Initialize the client.

        Args:
            config: SDK configuration
            api_key: API key (overrides config)
        """
        self.config = config
        self.api_key = api_key or config.api_key
        self.session = requests.Session()

        if self.api_key:
            self.session.headers["Authorization"] = f"Bearer {self.api_key}"

        self.session.headers["Content-Type"] = "application/json"
        self.session.headers["User-Agent"] = "p95-python/0.1.0"

    def _url(self, path: str) -> str:
        """Build full URL for an API endpoint."""
        return urljoin(self.config.base_url, f"/api/v1{path}")

    def _request(
        self,
        method: str,
        path: str,
        data: Optional[Dict[str, Any]] = None,
        params: Optional[Dict[str, Any]] = None,
    ) -> Dict[str, Any]:
        """
        Make an HTTP request with retry logic.

        Args:
            method: HTTP method
            path: API path
            data: Request body data
            params: Query parameters

        Returns:
            Response JSON data

        Raises:
            AuthenticationError: If authentication fails
            APIError: If the request fails
        """
        url = self._url(path)
        last_error = None

        for attempt in range(self.config.retry_count):
            try:
                response = self.session.request(
                    method=method,
                    url=url,
                    json=data,
                    params=params,
                    timeout=self.config.timeout,
                )

                if response.status_code == 401:
                    raise AuthenticationError("Invalid API key")

                if response.status_code == 403:
                    raise AuthenticationError("Insufficient permissions")

                if response.status_code >= 400:
                    error_data = response.json() if response.text else {}
                    raise APIError(
                        error_data.get(
                            "error",
                            f"Request failed with status {response.status_code}",
                        ),
                        status_code=response.status_code,
                        response=error_data,
                    )

                if response.text:
                    return response.json()
                return {}

            except requests.exceptions.RequestException as e:
                last_error = e
                if attempt < self.config.retry_count - 1:
                    time.sleep(self.config.retry_delay * (attempt + 1))
                continue

        raise APIError(
            f"Request failed after {self.config.retry_count} attempts: {last_error}"
        )

    def create_run(
        self,
        team_slug: str,
        app_slug: str,
        name: Optional[str] = None,
        tags: Optional[List[str]] = None,
        config: Optional[Dict[str, Any]] = None,
        git_info: Optional[Dict[str, Any]] = None,
        system_info: Optional[Dict[str, Any]] = None,
    ) -> str:
        """
        Create a new run.

        Returns:
            The run ID
        """
        data = {
            "name": name,
            "tags": tags or [],
            "config": config or {},
        }

        if git_info:
            data["git_info"] = git_info
        if system_info:
            data["system_info"] = system_info

        response = self._request(
            "POST",
            f"/teams/{team_slug}/apps/{app_slug}/runs",
            data=data,
        )

        return response["id"]

    def update_run_status(
        self,
        run_id: str,
        status: str,
        error: Optional[str] = None,
    ) -> None:
        """Update run status (completed, failed, aborted)."""
        data = {"status": status}
        if error:
            data["error_message"] = error

        self._request("PUT", f"/runs/{run_id}/status", data=data)

    def update_run_config(self, run_id: str, config: Dict[str, Any]) -> None:
        """Merge new config with existing run config."""
        self._request("PUT", f"/runs/{run_id}/config", data=config)

    def batch_log_metrics(
        self,
        run_id: str,
        metrics: List[Dict[str, Any]],
    ) -> None:
        """
        Log a batch of metrics.

        Args:
            run_id: The run ID
            metrics: List of metric dictionaries with name, value, step, timestamp
        """
        self._request(
            "POST",
            f"/runs/{run_id}/metrics",
            data={"metrics": metrics},
        )

    def add_run_tags(self, run_id: str, tags: List[str]) -> None:
        """Add tags to a run."""
        # This would need to be implemented via the update endpoint
        pass

    def upload_artifact(
        self,
        run_id: str,
        path: str,
        name: Optional[str] = None,
        metadata: Optional[Dict[str, Any]] = None,
    ) -> None:
        """Upload an artifact file."""
        # Artifact upload would need multipart form handling
        # For now, this is a placeholder
        pass

    def resume_run(
        self,
        run_id: str,
        config: Optional[Dict[str, Any]] = None,
        note: Optional[str] = None,
        git_info: Optional[Dict[str, Any]] = None,
        system_info: Optional[Dict[str, Any]] = None,
    ) -> Dict[str, Any]:
        """
        Resume a previously completed/failed run.

        Args:
            run_id: The run ID to resume
            config: New/updated config (merged with existing)
            note: Optional continuation note
            git_info: Git information at resume time
            system_info: System information at resume time

        Returns:
            Dictionary with 'run' and 'continuation' keys
        """
        data: Dict[str, Any] = {}
        if config:
            data["config"] = config
        if note:
            data["note"] = note
        if git_info:
            data["git_info"] = git_info
        if system_info:
            data["system_info"] = system_info

        return self._request("POST", f"/runs/{run_id}/resume", data=data)

    def get_continuations(self, run_id: str) -> List[Dict[str, Any]]:
        """
        Get all continuations for a run.

        Args:
            run_id: The run ID

        Returns:
            List of continuation dictionaries
        """
        response = self._request("GET", f"/runs/{run_id}/continuations")
        return response.get("continuations", [])

    def get_pending_intervention(self, run_id: str) -> Optional[Dict[str, Any]]:
        """
        Get the pending intervention for a run, if any.

        Args:
            run_id: The run ID

        Returns:
            Intervention dictionary or None if no pending intervention
        """
        response = self._request("GET", f"/runs/{run_id}/intervention/pending")
        return response.get("intervention")

    def ack_intervention(self, intervention_id: str) -> Dict[str, Any]:
        """
        Acknowledge and apply an intervention.

        Args:
            intervention_id: The intervention ID

        Returns:
            Updated intervention dictionary
        """
        return self._request("POST", f"/interventions/{intervention_id}/apply")
