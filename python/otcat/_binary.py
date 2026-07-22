"""Locates the otcat binary this package wraps.

Resolution order, first match wins:

1. ``OTCAT_BINARY`` environment variable, if set (an explicit escape
   hatch -- useful for pointing at a locally built binary during otcat
   Go-side development, or a version other than the one bundled here).
2. A binary bundled inside this wheel, under ``otcat/_bin/``, matching
   the current OS and CPU architecture. Platform-specific wheels (see
   ``docs/packaging.md`` in the Python package) ship exactly one such
   binary; this source checkout ships only ``otcat-linux-amd64``,
   enough to develop and test on the platform this project's own CI
   runs on.
3. ``otcat`` or ``otc`` on ``$PATH`` -- covers the case where the Go
   binary was installed separately (``go install``, a system package
   from Cloudsmith, a manual download).

If none of these resolve, :class:`OtcatBinaryNotFoundError` explains
exactly what was tried, so a confusing "file not found" from deep
inside subprocess machinery never reaches the caller.
"""
from __future__ import annotations

import os
import platform
import shutil
import stat
from pathlib import Path

from .exceptions import OtcatBinaryNotFoundError

_BIN_DIR = Path(__file__).parent / "_bin"


def _platform_tag() -> str:
    system = platform.system().lower()
    machine = platform.machine().lower()

    os_name = {"darwin": "darwin", "linux": "linux", "windows": "windows"}.get(system, system)
    arch = {
        "x86_64": "amd64", "amd64": "amd64",
        "aarch64": "arm64", "arm64": "arm64",
    }.get(machine, machine)
    return f"{os_name}-{arch}"


def _bundled_path() -> Path | None:
    tag = _platform_tag()
    suffix = ".exe" if tag.startswith("windows") else ""
    candidate = _BIN_DIR / f"otcat-{tag}{suffix}"
    return candidate if candidate.is_file() else None


def find_binary() -> str:
    """Return a path to a working otcat binary, or raise
    :class:`OtcatBinaryNotFoundError` with a clear explanation."""
    tried: list[str] = []

    env = os.environ.get("OTCAT_BINARY")
    if env:
        tried.append(f"$OTCAT_BINARY={env}")
        if os.path.isfile(env) and os.access(env, os.X_OK):
            return env

    bundled = _bundled_path()
    if bundled:
        tried.append(f"bundled binary at {bundled}")
        if not os.access(bundled, os.X_OK):
            # Wheel installers don't always preserve the executable
            # bit; fix it rather than fail on something this
            # trivially fixable.
            bundled.chmod(bundled.stat().st_mode | stat.S_IEXEC | stat.S_IXGRP | stat.S_IXOTH)
        return str(bundled)
    else:
        tried.append(f"bundled binary for platform '{_platform_tag()}' (none shipped in this install)")

    for name in ("otcat", "otc"):
        found = shutil.which(name)
        tried.append(f"'{name}' on $PATH")
        if found:
            return found

    raise OtcatBinaryNotFoundError(tried)
