"""Tests for config priority: configure() should win over environment variables."""

import importlib

import pytest


def reload_config():
    """Reload the config module to reset global state between tests."""
    import p95.config as config_mod

    importlib.reload(config_mod)
    return config_mod


def test_env_var_overrides_default(monkeypatch):
    """P95_LOGDIR env var switches mode to local (sanity check)."""
    monkeypatch.setenv("P95_LOGDIR", "/tmp/logs")
    monkeypatch.delenv("P95_URL", raising=False)
    monkeypatch.delenv("P95_API_KEY", raising=False)

    config_mod = reload_config()
    config = config_mod.get_config()

    assert config.mode == "local"


def test_env_var_remote_detection(monkeypatch):
    """P95_URL env var switches mode to remote."""
    monkeypatch.setenv("P95_URL", "http://example.com")
    monkeypatch.setenv("P95_API_KEY", "test-key")
    monkeypatch.delenv("P95_LOGDIR", raising=False)

    config_mod = reload_config()
    config = config_mod.get_config()

    assert config.mode == "remote"


def test_configure_takes_priority_over_env_var(monkeypatch):
    """
    configure(mode=...) must take priority over environment variables.
    """
    monkeypatch.setenv("P95_LOGDIR", "/tmp/logs")
    monkeypatch.delenv("P95_URL", raising=False)
    monkeypatch.delenv("P95_API_KEY", raising=False)

    config_mod = reload_config()
    config_mod.configure(mode="remote", base_url="http://example.com", api_key="key")
    config = config_mod.get_config()

    assert config.mode == "remote"


@pytest.mark.parametrize(
    ("configure_kwargs", "env_var", "env_value", "expected_attr", "expected_value"),
    [
        ({"mode": "remote"}, "P95_LOGDIR", "/tmp/logs", "mode", "remote"),
        (
            {"logdir": "/custom/logs"},
            "P95_LOGDIR",
            "/tmp/logs",
            "logdir",
            "/custom/logs",
        ),
        (
            {"base_url": "http://configured.example.com"},
            "P95_URL",
            "http://env.example.com",
            "base_url",
            "http://configured.example.com",
        ),
        (
            {"api_key": "configured-key"},
            "P95_API_KEY",
            "env-key",
            "api_key",
            "configured-key",
        ),
    ],
)
def test_configure_field_takes_priority_over_matching_env(
    monkeypatch, configure_kwargs, env_var, env_value, expected_attr, expected_value
):
    """Fields explicitly set via configure() should not be overridden by env vars."""
    monkeypatch.setenv(env_var, env_value)
    monkeypatch.delenv("P95_LOGDIR", raising=False)
    monkeypatch.delenv("P95_URL", raising=False)
    monkeypatch.delenv("P95_API_KEY", raising=False)
    monkeypatch.setenv(env_var, env_value)

    config_mod = reload_config()
    config_mod.configure(**configure_kwargs)
    config = config_mod.get_config()

    assert getattr(config, expected_attr) == expected_value


def test_unset_fields_still_populate_from_env_when_some_fields_configured(monkeypatch):
    """
    Explicitly setting one field should not block env vars for other unset fields.
    """
    monkeypatch.setenv("P95_URL", "http://env.example.com")
    monkeypatch.setenv("P95_API_KEY", "env-key")
    monkeypatch.delenv("P95_LOGDIR", raising=False)

    config_mod = reload_config()
    config_mod.configure(base_url="http://configured.example.com")
    config = config_mod.get_config()

    assert config.base_url == "http://configured.example.com"
    assert config.api_key == "env-key"
    assert config.mode == "remote"
