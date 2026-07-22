from __future__ import annotations

import pytest

from otcat import ProtocolError, WriteAbortedError
from otcat.aio import AsyncClient

pytestmark = pytest.mark.asyncio


async def test_async_read(mockplc, otcat_env):
    c = AsyncClient(mockplc, raw_address=True)
    v = await c.read("holding:100")
    assert v.value == 0x1234


async def test_async_write_and_readback(mockplc, otcat_env):
    c = AsyncClient(mockplc, raw_address=True)
    await c.write("holding:50", 4242)
    v = await c.read("holding:50")
    assert v.value == 4242


async def test_async_write_confirm_false_refused(mockplc, otcat_env):
    c = AsyncClient(mockplc, raw_address=True)
    with pytest.raises(WriteAbortedError):
        await c.write("holding:50", 1, confirm=False)


async def test_async_dry_run(mockplc, otcat_env):
    c = AsyncClient(mockplc, raw_address=True)
    plan = await c.dry_run("holding:100", 555)
    assert plan.registers == [555]


async def test_async_watch(mockplc, otcat_env):
    c = AsyncClient(mockplc, raw_address=True)
    values = []
    async for v in c.watch("holding:0", interval="50ms", count=4):
        values.append(v)
    assert len(values) == 4


async def test_async_protocol_error(mockplc, otcat_env):
    c = AsyncClient(mockplc, raw_address=True)
    with pytest.raises(ProtocolError):
        await c.read("holding:65530:10")
