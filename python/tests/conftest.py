"""Fixtures shared by the whole test suite.

These tests are integration tests by design, matching the Go core's
own testing philosophy (see the paper's Verification Methodology
section): every test in this package runs the real, compiled otcat
binary against a real, compiled otcat-mockplc server over an actual
TCP socket on loopback. Nothing here mocks the Python client itself --
that would only prove the Python code agrees with its own assumptions,
exactly the blind spot the Go core's paper explicitly calls out.
"""
from __future__ import annotations

import os
import shutil
import socket
import subprocess
import sys
import time
from pathlib import Path

import pytest

REPO_ROOT = Path(__file__).resolve().parents[2]
PYTHON_PKG_ROOT = Path(__file__).resolve().parents[1]


def _go_bin() -> str:
    for candidate in ("go", "/usr/lib/go-1.22/bin/go", "/usr/lib/go-1.24/bin/go"):
        found = shutil.which(candidate) if candidate == "go" else (candidate if os.path.isfile(candidate) else None)
        if found:
            return found
    pytest.skip("no Go toolchain found; cannot build test fixtures (mockplc, otcat)")


def _build_once(cache_dir: Path, name: str, pkg: str) -> str:
    cache_dir.mkdir(parents=True, exist_ok=True)
    out = cache_dir / name
    if not out.exists():
        go = _go_bin()
        subprocess.run(
            [go, "build", "-o", str(out), pkg],
            cwd=REPO_ROOT, check=True, capture_output=True, text=True,
        )
    return str(out)


@pytest.fixture(scope="session")
def otcat_binary() -> str:
    """Prefer the bundled binary this package ships (proves the actual
    packaged artifact works, not just a freshly built one), falling
    back to a fresh build for platforms/dev setups without a bundled
    binary for the current OS/arch."""
    bundled = PYTHON_PKG_ROOT / "otcat" / "_bin" / "otcat-linux-amd64"
    if bundled.is_file() and sys.platform.startswith("linux"):
        os.chmod(bundled, 0o755)
        return str(bundled)
    return _build_once(Path("/tmp/otcat_py_test_bin"), "otcat", "./cmd/otcat")


@pytest.fixture(scope="session")
def mockplc_binary() -> str:
    return _build_once(Path("/tmp/otcat_py_test_bin"), "otcat-mockplc", "./cmd/otcat-mockplc")


def _free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


@pytest.fixture
def mockplc(mockplc_binary):
    """Starts a fresh otcat-mockplc for each test on its own port
    (avoiding any cross-test state, notably the tank-level simulator's
    time-varying holding:0) and guarantees it's actually accepting
    connections before the test body runs."""
    port = _free_port()
    addr = f"127.0.0.1:{port}"
    proc = subprocess.Popen(
        [mockplc_binary, "--addr", addr],
        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
        stdin=subprocess.DEVNULL,
    )
    try:
        deadline = time.time() + 5
        while time.time() < deadline:
            try:
                with socket.create_connection(("127.0.0.1", port), timeout=0.2):
                    break
            except OSError:
                time.sleep(0.05)
        else:
            proc.kill()
            pytest.fail(f"mockplc on {addr} never accepted a connection")
        yield addr
    finally:
        proc.kill()
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            pass


@pytest.fixture
def otcat_env(monkeypatch, otcat_binary):
    """Points OTCAT_BINARY at the resolved test binary so Client()
    calls in tests don't depend on $PATH."""
    monkeypatch.setenv("OTCAT_BINARY", otcat_binary)
