"""Async twin of :class:`otcat.client.Client`, built on
``asyncio.create_subprocess_exec`` instead of the synchronous
``subprocess`` module, so it doesn't block an event loop -- the
relevant case being a FastAPI endpoint that reads or watches a
register without stalling every other request being served by the
same process. Same flags, same JSON parsing, same exception mapping;
see client.py's module docstring for the design rationale, which
applies here unchanged.
"""
from __future__ import annotations

import asyncio
import json
from collections.abc import AsyncIterator
from typing import Any

from . import _binary
from .exceptions import Timeout, error_for_exit_code
from .models import Value, WritePlan

_DEFAULT_TIMEOUT = 5.0


class AsyncClient:
    def __init__(
        self,
        endpoint: str,
        *,
        binary: str | None = None,
        unit: int = 1,
        type: str = "uint16",  # noqa: A002
        byte_order: str = "big",
        word_order: str = "high",
        raw_address: bool = False,
        timeout: float = _DEFAULT_TIMEOUT,
    ) -> None:
        self.endpoint = endpoint
        self.binary = binary or _binary.find_binary()
        self.unit = unit
        self.type = type
        self.byte_order = byte_order
        self.word_order = word_order
        self.raw_address = raw_address
        self.timeout = timeout

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

    async def _run(self, args: list[str]) -> tuple[int, str, str]:
        proc = await asyncio.create_subprocess_exec(
            self.binary, *args,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )
        try:
            stdout, stderr = await asyncio.wait_for(
                proc.communicate(), timeout=self.timeout + 5
            )
        except asyncio.TimeoutError as e:
            proc.kill()
            await proc.wait()
            raise Timeout(
                f"otcat process did not exit within {self.timeout + 5}s"
            ) from e
        assert proc.returncode is not None  # guaranteed once communicate() has returned
        return proc.returncode, stdout.decode(), stderr.decode()

    async def read(self, spec: str) -> Value:
        code, out, err = await self._run([*self._base_flags(), "--read", spec])
        if code != 0:
            raise error_for_exit_code(code, err)
        return Value.from_json(json.loads(out.strip().splitlines()[-1]))

    async def write(self, spec: str, value: Any, *, confirm: bool = True) -> None:
        args = [*self._base_flags(), "--write", spec, "--value", str(value)]
        if confirm:
            args.append("--confirm")
        code, _out, err = await self._run(args)
        if code != 0:
            raise error_for_exit_code(code, err)

    async def dry_run(self, spec: str, value: Any) -> WritePlan:
        args = [*self._base_flags(), "--write", spec, "--value", str(value), "--dry-run"]
        code, out, err = await self._run(args)
        if code != 0:
            raise error_for_exit_code(code, err)
        return WritePlan.from_json(json.loads(out.strip().splitlines()[-1]))

    async def watch(
        self, spec: str, *, interval: str = "1s", count: int = 0
    ) -> AsyncIterator[Value]:
        """Async generator streaming readings -- `async for v in
        client.watch(...)`. Ideal for a FastAPI WebSocket endpoint
        forwarding live register values to a browser."""
        args = [
            *self._base_flags(),
            "--watch", spec,
            "--interval", interval,
            "--count", str(count),
        ]
        proc = await asyncio.create_subprocess_exec(
            self.binary, *args,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )
        try:
            assert proc.stdout is not None
            async for raw_line in proc.stdout:
                line = raw_line.decode().strip()
                if not line:
                    continue
                yield Value.from_json(json.loads(line))
        finally:
            if proc.returncode is None:
                proc.terminate()
                try:
                    await asyncio.wait_for(proc.wait(), timeout=3)
                except asyncio.TimeoutError:
                    proc.kill()
                    await proc.wait()

    def __repr__(self) -> str:
        return f"AsyncClient(endpoint={self.endpoint!r}, unit={self.unit}, type={self.type!r})"
