"""pandas integration. Kept as a separate, optional-import module
(pandas is not a hard dependency of otcat's core client -- see
pyproject.toml's [project.optional-dependencies]) so anything that
only needs Client/AsyncClient never has to install pandas at all.
"""
from __future__ import annotations

from collections.abc import Iterable
from typing import TYPE_CHECKING

from .client import Client
from .models import Value

if TYPE_CHECKING:
    import pandas as pd

_MISSING_PANDAS = (
    "pandas is required for otcat.pandas_ext but is not installed. "
    "Install it with: pip install otcat[pandas]"
)


def _require_pandas():
    try:
        import pandas as pd
    except ImportError as e:  # pragma: no cover - exercised only without pandas installed
        raise ImportError(_MISSING_PANDAS) from e
    return pd


def values_to_dataframe(values: Iterable[Value]) -> "pd.DataFrame":
    """Convert any iterable of Value (e.g. Client.read_many's return,
    or a fully-consumed Client.watch() generator) into a DataFrame with
    columns [address, type, value, quality, ts] and ts as the index --
    ready for `.resample()`, `.rolling()`, or a straight `.plot()`."""
    pd = _require_pandas()
    rows = [
        {
            "ts": v.ts,
            "address": v.address,
            "type": v.type,
            "value": v.value,
            "quality": v.quality,
        }
        for v in values
    ]
    df = pd.DataFrame(rows)
    if not df.empty:
        df = df.set_index("ts")
    return df


def read_many_df(client: Client, specs: list[str]) -> "pd.DataFrame":
    """client.read_many(specs), as a DataFrame. One row per spec."""
    return values_to_dataframe(client.read_many(specs))


def watch_df(
    client: Client,
    spec: str,
    *,
    count: int,
    interval: str = "1s",
) -> "pd.DataFrame":
    """Block until `count` samples are collected via client.watch(),
    then return them as one DataFrame. For an unbounded live stream
    instead, iterate client.watch() directly and accumulate/append
    however your application (a dashboard, a rolling buffer) wants --
    a single ever-growing DataFrame is usually the wrong shape for a
    truly unbounded stream, which is why this helper requires a finite
    count rather than offering an unbounded DataFrame-producing mode."""
    values = list(client.watch(spec, interval=interval, count=count))
    return values_to_dataframe(values)


class RollingBuffer:
    """A fixed-size rolling window over a live watch() stream,
    materialized as a DataFrame on demand -- the shape a live
    dashboard or a streaming feature-engineering step usually wants:
    bounded memory, always-current, no unbounded accumulation.

    >>> client = Client("127.0.0.1:502")
    >>> buf = RollingBuffer(maxlen=500)
    >>> for v in client.watch("holding:0", interval="200ms"):
    ...     buf.push(v)
    ...     if len(buf) >= 10:
    ...         df = buf.to_dataframe()
    ...         # df.value.rolling(5).mean(), df.plot(), feed a model, etc.
    """

    def __init__(self, maxlen: int = 1000):
        from collections import deque
        self.maxlen = maxlen
        self._buf: deque[Value] = deque(maxlen=maxlen)

    def push(self, value: Value) -> None:
        self._buf.append(value)

    def __len__(self) -> int:
        return len(self._buf)

    def to_dataframe(self) -> "pd.DataFrame":
        return values_to_dataframe(list(self._buf))
