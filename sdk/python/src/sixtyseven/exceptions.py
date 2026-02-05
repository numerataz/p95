"""Custom exceptions for the Sixtyseven SDK."""


class SixtySevenError(Exception):
    """Base exception for Sixtyseven SDK errors."""

    pass


class AuthenticationError(SixtySevenError):
    """Raised when authentication fails."""

    pass


class APIError(SixtySevenError):
    """Raised when an API request fails."""

    def __init__(self, message: str, status_code: int = None, response: dict = None):
        super().__init__(message)
        self.status_code = status_code
        self.response = response


class ValidationError(SixtySevenError):
    """Raised when validation fails."""

    pass


class ConnectionError(SixtySevenError):
    """Raised when connection to the server fails."""

    pass


class ServerError(SixtySevenError):
    """Raised when server management fails (e.g., binary not found, failed to start)."""

    pass
