"""Custom exceptions for the p95 SDK."""


class P95Error(Exception):
    """Base exception for p95 SDK errors."""

    pass


class AuthenticationError(P95Error):
    """Raised when authentication fails."""

    pass


class APIError(P95Error):
    """Raised when an API request fails."""

    def __init__(self, message: str, status_code: int = None, response: dict = None):
        super().__init__(message)
        self.status_code = status_code
        self.response = response


class ValidationError(P95Error):
    """Raised when validation fails."""

    pass


class ConnectionError(P95Error):
    """Raised when connection to the server fails."""

    pass


class ServerError(P95Error):
    """Raised when server management fails (e.g., binary not found, failed to start)."""

    pass


class EarlyStopException(P95Error):
    """Raised when an early stop intervention is applied.

    Training loops should catch this exception to gracefully exit.

    Example:
        try:
            for epoch in range(100):
                train_step()
                run.apply_intervention(run.check_intervention())
        except EarlyStopException as e:
            print(f"Early stop: {e.rationale}")
    """

    def __init__(self, rationale: str):
        super().__init__(f"Early stop requested: {rationale}")
        self.rationale = rationale
