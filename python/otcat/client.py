"""The synchronous otcat client.

This is deliberately a subprocess wrapper around the real, compiled Go
binary -- not a reimplementation of the Modbus protocol in Python.
Every byte that crosses a real socket is still produced and parsed by
the same Go core the paper describes and the Go test suite exercises;
this package's only job is turning that binary's ndjson stdout into
typed Python objects, and Python calls into the right CLI flags.

Why subprocess and not a cgo shared library / ctypes binding: it is
the integration surface otcat was already designed around (stdin,
stdout, one ndjson object per line), it costs nothing extra to keep
correct as the Go core evolves, and it works on every platform the Go
binary itself supports with zero additional build tooling on the
Python side. The cost is one process-spawn per read/write and a few
milliseconds of overhead -- immaterial next to a Modbus round trip
(see the paper's own latency figures) and irrelevant for --watch,
which spawns once and streams.

Write safety: exactly one behavior carries over from the CLI without
a Python-side opinion layered on top -- Client.write() defaults to
confirm=True, deliberately the *opposite* default of the CLI's own
--confirm flag. The CLI defaults to refusing because a human might be
about to fat-finger a command; a library call is already the result of
a programmer deciding, in code, to write a specific value -- there is
no keystroke left to protect against. If you want the CLI's stricter
default in your own code, pass confirm=False and handle
WriteAbortedError.
"""
from __future__ import annotations

import json
import subprocess
from collections.abc import Iterator
from typing import Any

from . import _binary
from .exceptions import Timeout, error_for_exit_code
from .models import Value, WritePlan

_DEFAULT_TIMEOUT = 5.0


class Client:
    def __init__(
        self,
        endpoint: str,
        *,
        binary: str | None = None,
        unit: int = 1,
        type: str = "uint16",  # noqa: A002 - matches the CLI's --type name
        byte_order: str = "big",
        word_order: str = "high",
        raw_address: bool = False,
        timeout: float = _DEFAULT_TIMEOUT,
    ) -> None:
        """endpoint: "host:port" of a Modbus TCP device or otcat-mockplc."""
        self.endpoint = endpoint
        self.binary = binary or _binary.find_binary()
        self.unit = unit
        self.type = type
        self.byte_order = byte_order
        self.word_order = word_order
        self.raw_address = raw_address
        self.timeout = timeout

    # -- shared flag building -------------------------------------------------

    def _base_flags(self) -> list[str]:
        flags = [
            "--modbus", self.endpoint,
            "--unit", str(self.unit),
            "--type", self.type,
            "--byte-order", self.byte_order,
            "--word-order", self.word_order,
            "--timeout", f"{self.timeout}s",
            "--json",
        ]
        if self.raw_address:
            flags.append("--raw-address")
        return flags

    def _run(self, args: list[str], input_text: str | None = None) -> subprocess.CompletedProcess:
        try:
            return subprocess.run(
                [self.binary, *args],
                input=input_text,
                capture_output=True,
                text=True,
                timeout=self.timeout + 5,  # generous margin over otcat's own --timeout
            )
        except subprocess.TimeoutExpired as e:
            raise Timeout(
                f"otcat process did not exit within {self.timeout + 5}s "
                f"(otcat's own --timeout is {self.timeout}s; this is the "
                f"Python-side watchdog on top of it)"
            ) from e

    # -- public API -------------------------------------------------------------

    def read(self, spec: str) -> Value:
        """Read one address once. spec: e.g. "holding:40001" or
        "holding:200:2" for a 2-register array."""
        proc = self._run([*self._base_flags(), "--read", spec])
        if proc.returncode != 0:
            raise error_for_exit_code(proc.returncode, proc.stderr)
        return Value.from_json(json.loads(proc.stdout.strip().splitlines()[-1]))

    def read_many(self, specs: list[str]) -> list[Value]:
        """Convenience: read() each spec in sequence (N processes, N
        round trips -- not a single batched Modbus request, since
        otcat's own protocol layer doesn't batch either; see the
        paper's discussion of why the client stays single-request)."""
        return [self.read(spec) for spec in specs]

    def write(
        self,
        spec: str,
        value: Any,
        *,
        confirm: bool = True,
    ) -> None:
        """Write value to spec. See the module docstring for why
        confirm defaults to True here, opposite the CLI's own default."""
        args = [*self._base_flags(), "--write", spec, "--value", str(value)]
        if confirm:
            args.append("--confirm")
        proc = self._run(args)
        if proc.returncode != 0:
            raise error_for_exit_code(proc.returncode, proc.stderr)

    def dry_run(self, spec: str, value: Any) -> WritePlan:
        """Compute exactly what write(spec, value) would send, with
        zero network I/O -- see internal/cliapp's --dry-run in the Go
        core; this calls the identical code path."""
        args = [*self._base_flags(), "--write", spec, "--value", str(value), "--dry-run"]
        proc = self._run(args)
        if proc.returncode != 0:
            raise error_for_exit_code(proc.returncode, proc.stderr)
        return WritePlan.from_json(json.loads(proc.stdout.strip().splitlines()[-1]))

    def watch(
        self,
        spec: str,
        *,
        interval: str = "1s",
        count: int = 0,
    ) -> Iterator[Value]:
        """Stream readings as they arrive. A generator wrapping one
        long-lived `otcat --watch` subprocess -- values are yielded the
        instant otcat's own stdout flushes them (line-buffered, exactly
        as documented in the Go codec package), not batched.

        count=0 (the default) streams until the caller stops iterating
        (breaking out of a `for` loop terminates the subprocess
        cleanly) or the process is otherwise interrupted.
        """
        args = [
            *self._base_flags(),
            "--watch", spec,
            "--interval", interval,
            "--count", str(count),
        ]
        proc = subprocess.Popen(
            [self.binary, *args],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1,  # line-buffered
        )
        try:
            assert proc.stdout is not None
            for line in proc.stdout:
                line = line.strip()
                if not line:
                    continue
                yield Value.from_json(json.loads(line))
        finally:
            if proc.poll() is None:
                proc.terminate()
                try:
                    proc.wait(timeout=3)
                except subprocess.TimeoutExpired:
                    proc.kill()
                    proc.wait()

    def __repr__(self) -> str:
        return f"Client(endpoint={self.endpoint!r}, unit={self.unit}, type={self.type!r})"
