from __future__ import annotations

import os
from pathlib import Path

from setuptools import setup


def _is_musl() -> bool:
    """Detect if running on musl libc (Alpine Linux)."""
    # Check for Alpine
    if Path("/etc/alpine-release").exists():
        return True
    # Check AUDITWHEEL_PLAT env var set by cibuildwheel
    plat = os.environ.get("AUDITWHEEL_PLAT", "")
    if "musllinux" in plat:
        return True
    return False


try:
    from wheel.bdist_wheel import bdist_wheel as _bdist_wheel

    class bdist_wheel(_bdist_wheel):
        def finalize_options(self) -> None:
            super().finalize_options()
            self.root_is_pure = False
            if hasattr(self, "root_is_purelib"):
                self.root_is_purelib = False

        def get_tag(self) -> tuple[str, str, str]:
            _, _, plat = super().get_tag()
            # py3-none-<platform>: works with any Python 3, no ABI, platform-specific
            # The Go binary is statically linked so it's portable across Linux distros
            if plat == "linux_x86_64":
                if _is_musl():
                    plat = "musllinux_1_2_x86_64"
                else:
                    plat = "manylinux2014_x86_64"
            elif plat == "linux_aarch64":
                if _is_musl():
                    plat = "musllinux_1_2_aarch64"
                else:
                    plat = "manylinux2014_aarch64"
            return "py3", "none", plat

except Exception:  # wheel not available
    bdist_wheel = None  # type: ignore[assignment]


setup(cmdclass={"bdist_wheel": bdist_wheel} if bdist_wheel else {})
