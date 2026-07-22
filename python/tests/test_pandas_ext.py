from __future__ import annotations

import pytest

pd = pytest.importorskip("pandas")

from otcat import Client
from otcat.pandas_ext import RollingBuffer, read_many_df, values_to_dataframe, watch_df


def test_read_many_df(mockplc, otcat_env):
    c = Client(mockplc, raw_address=True)
    df = read_many_df(c, ["holding:100", "input:5"])
    assert len(df) == 2
    assert "value" in df.columns
    assert df.index.name == "ts"


def test_watch_df(mockplc, otcat_env):
    c = Client(mockplc, raw_address=True)
    df = watch_df(c, "holding:0", count=5, interval="40ms")
    assert len(df) == 5
    # the tank level is monotonically non-decreasing over this short window
    assert df["value"].is_monotonic_increasing or df["value"].nunique() == 1


def test_empty_dataframe_shape(mockplc, otcat_env):
    df = values_to_dataframe([])
    assert len(df) == 0


def test_rolling_buffer(mockplc, otcat_env):
    c = Client(mockplc, raw_address=True)
    buf = RollingBuffer(maxlen=3)
    for v in c.watch("holding:0", interval="30ms", count=5):
        buf.push(v)
    assert len(buf) == 3  # bounded even though 5 values were pushed
    df = buf.to_dataframe()
    assert len(df) == 3
